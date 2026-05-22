# Inventario — Security Threat Model

Reference document for the `security-reviewer` Claude Code subagent
(`.claude/agents/security-reviewer.md`). It defines the project-specific
invariants a generic security auditor would miss, plus the false-positive list
of behaviors that are intentional and must not be flagged.

The agent applies Anthropic's `claude-code-security-review` *methodology*
(>80%-confidence findings, exploitability-driven HIGH/MEDIUM/LOW severity, the
standard exclusions for DoS / rate limiting / secrets-on-disk) and this file
supplies the *Inventario knowledge* it runs that methodology against.

---

## Architecture in one paragraph

Inventario is a multi-tenant inventory SaaS. The Go backend (`go/`) serves a REST
API under `/api/v1`. Every tenant's data is isolated by **two independent
layers**: (a) Postgres RLS via a per-transaction GUC (`set_tenant_context`), and
(b) registry factories that bind `tenantID` at construction so all queries carry
`WHERE tenant_id = $n`. The React frontend (`frontend/`) is a SPA. A separate
`/api/v1/admin/*` subtree is **deliberately cross-tenant** and gated only by the
`RequireSystemAdmin` middleware. Treat any change that weakens either isolation
layer, or that widens the admin subtree, as HIGH severity.

---

## 1. Tenant isolation — the crown jewel

Flag as **HIGH** if a change can let a request read or mutate another tenant's data.

- A new registry method or query that does **not** filter by `tenant_id`, or that
  takes a tenant ID from user input instead of from `appctx.UserFromContext`.
- A handler that accepts a tenant ID from a header, query param, or request body.
  The middleware `ValidateNoUserProvidedTenantID` / `RejectSpecificTenantHeaders`
  (`go/apiserver/security_middleware.go`) must reject these — a new route that
  bypasses that chain is a finding.
- Use of a **service registry** (cross-tenant) where a **user registry**
  (tenant-scoped) is correct. Service registries bypass RLS by design; using one
  on a non-admin path is a tenant escape.
- A new route mounted under `/api/v1/admin/` **without** `RequireSystemAdmin`,
  or a non-admin route that is wrongly added to the `isAdminSubtreePath`
  exemption in `security_middleware.go`.
- RLS GUC set with `SET` instead of `SET LOCAL`, or outside a transaction —
  this leaks tenant context across pooled (pgbouncer) connections.

## 2. Authentication & authorization

- JWT validated with the wrong algorithm, a missing signature check, or `alg:none`
  accepted. Verify `golang-jwt` parsing pins HMAC.
- A handler reachable without `RequireAuth` that exposes tenant or user data.
- **Impersonation** (`go/apiserver/admin_impersonation.go`): an impersonation
  access token must carry `is_system_admin=false` and `imp=true`. Flag any path
  that lets an impersonated session reach a `RequireSystemAdmin` handler, or that
  permits nested impersonation (impersonating while impersonating).
- Authorization decided on a value from the token *body* that the client could
  influence, rather than on server-side state.
- Privilege checks done in the frontend only, with no backend enforcement.

## 3. CSRF & signed URLs

- A state-changing endpoint (POST/PUT/PATCH/DELETE) that escapes `CSRFMiddleware`.
- Signed-URL validation (`signed_url_middleware.go`) that skips the HMAC check,
  the expiry check, or the `file.TenantID == user.TenantID` cross-tenant check.
- HMAC compared with `==` instead of a constant-time compare (`hmac.Equal`).
- Signing secret read from a predictable/empty default.

## 4. SQL & registries

Registries use parameterized queries (`$1, $2, …`); `fmt.Sprintf` is used **only**
for static table/column identifiers.

- Flag **HIGH** if any user-controlled value reaches `fmt.Sprintf`, string
  concatenation, or an identifier position in a query.
- A dynamic `WHERE`/`ORDER BY` builder (e.g. `buildCommodityWhere`) where a sort
  column, direction, or filter key comes from the request without an allowlist.
- A new advisory-lock site (`pg_advisory_xact_lock`) whose lock key is derived
  from untrusted input in a way that allows collision or lock-key forging.

## 5. File upload & serving

- Upload paths that trust a client-supplied filename, content-type, or path
  segment without sanitization (path traversal into the blob store).
- File download/thumbnail paths that resolve a file by ID without re-checking
  tenant ownership (the signed-URL check is defense-in-depth, not the only gate).
- Pre-signed upload slots (`uploads.go`) reusable, not expiry-bound, or not
  tenant-scoped.

## 6. Frontend

- Rendering server/user data via `dangerouslySetInnerHTML`, or building a DOM
  sink (`innerHTML`, `eval`, dynamic `<script>`) from API data — XSS that could
  exfiltrate the access token held in `localStorage` (`inventario_token`).
- The access token or CSRF token logged, put in a URL, or sent to a third-party
  origin.
- `RequireSystemAdmin` guard removed or weakened on an `/admin/*` route.
- API base URL or auth header sent cross-origin to an unintended host.

## 7. Error handling & information disclosure

- New code must use `errx` / `errxtrace` (never the deprecated `internal/errkit`).
- Internal errors, stack traces, SQL text, or tenant/user IDs leaked into an HTTP
  response body or a client-visible log.
- A sentinel-error mapping that turns an authz failure into a 200 / empty result
  instead of 403/404 (so callers can't distinguish "forbidden" from "absent" when
  that distinction itself leaks cross-tenant existence — judge case by case).

## 8. Configuration & secrets

- Hardcoded credentials, API keys, JWT/HMAC secrets, or DB passwords in source,
  test fixtures, Helm `values.yaml`, or workflow files.
- A new secret read with an insecure default (empty string, `"changeme"`).
- A Helm/k8s manifest mounting a secret into a world-readable path or logging it.

---

## Severity calibration

- **HIGH** — cross-tenant read/write, auth bypass, privilege escalation through
  impersonation, SQL injection, RCE, secret in source.
- **MEDIUM** — CSRF/signed-URL gap requiring a specific precondition, missing
  ownership re-check behind a still-present defense-in-depth layer, XSS in a
  low-traffic surface.
- **LOW** — defense-in-depth hardening, missing constant-time compare where
  timing is hard to exploit.

When in doubt about tenant isolation, err toward flagging — it is the one class
of bug this project cannot ship.

---

## False positives — do NOT flag (intentional design)

These are deliberate Inventario decisions. Do not surface them as findings unless
the change under review *newly introduces or widens* the behavior.

- **Fail-open token blacklist / CSRF service.** `services/token_blacklist.go`
  and `csrf_middleware.go` deliberately fail open when Redis/storage is
  unreachable, trading strict revocation for availability. Flag only if a *new*
  code path widens the open window (e.g. fails open on a non-storage error, or
  on the normal path).
- **`/api/v1/admin/*` is cross-tenant.** Admin handlers intentionally use
  service (non-RLS) registries and skip the user-supplied-tenant-ID query check.
  A missing `WHERE tenant_id = …` inside an admin handler is **not** a finding.
  A *missing `RequireSystemAdmin`* on such a route **is** a finding.
- **Access token in `localStorage`.** The frontend stores `inventario_token` in
  `localStorage` by project decision. Not a finding on its own — but do report
  any XSS sink that could read it.
- **`fmt.Sprintf` for table/column identifiers.** Registries format *static*
  identifiers into query strings while binding all user input via `$n`
  placeholders. Not SQL injection. Flag only when a **user-controlled** value
  reaches an identifier / `Sprintf` position.
- **Advisory locks for uniqueness.** `pg_advisory_xact_lock` in users/tags/
  group-membership registries is a correctness mechanism, not a vulnerability.
