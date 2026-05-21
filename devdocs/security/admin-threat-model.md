# Admin Section — Threat Model

Focused threat model for the system-wide admin section
([umbrella #1744](https://github.com/denisvmedia/inventario/issues/1744),
QA gate [#1758](https://github.com/denisvmedia/inventario/issues/1758)).
It covers the `/api/v1/admin/*` API subtree, the `/admin/*` UI, the
`is_system_admin` JWT claim, and the impersonation primitive.

Scope is deliberately narrow: it addresses what the admin section adds
on top of the existing auth stack (JWT + refresh tokens + CSRF + RLS).
General application threats are out of scope.

## Trust boundaries & assets

- **Assets**: every tenant's data (the admin surface is cross-tenant),
  the `is_system_admin` flag, the JWT signing secret, impersonation
  tokens, and the audit log.
- **Privileged principals**: system admins (`users.is_system_admin =
  true`) and anyone with database or host shell access (the CLI
  bootstrap path).
- **Boundary**: the `RequireSystemAdmin` middleware separates ordinary
  authenticated users from the admin surface; the database role's RLS
  policies separate tenants for all *non-admin* traffic.

---

## T1 — Privilege escalation: a non-admin self-grants admin

**Threat.** A regular user forges or mutates a token / record so that
`is_system_admin` reads `true`.

**Mitigations.**
- `is_system_admin` is a database column carrying the model tag
  `userinput:"false"` — it is never bound from a user-supplied request
  body, so no normal user/profile endpoint can set it.
- The flag is only ever written by the CLI (`admin grant-system-admin`)
  or the seed harness — both require direct database access.
- The JWT `is_system_admin` claim is signed (HS256) with the server
  secret; a tampered claim fails signature verification.
- `RequireSystemAdmin` re-reads the flag from the authenticated user
  context; a stale or self-asserted claim alone does not pass.

**Residual risk.** Compromise of the JWT signing secret (see T2) or of
the database. Both are pre-existing platform-level risks.

---

## T2 — JWT forgery / replay of admin or impersonation claims

**Threat.** An attacker self-signs a token with `is_system_admin: true`
or `imp: true` / `impersonated_by`, or replays a captured one.

**Mitigations.**
- All tokens are HS256-signed with the server secret and verified on
  every request; the algorithm is pinned (no `alg:none` downgrade).
- Access tokens are short-lived; impersonation tokens are shorter still
  (≤30 min, `INVENTARIO_IMPERSONATION_TTL`).
- Block bumps a per-user JWT-blacklist `iat`-staleness threshold, so a
  captured access token for a blocked user is rejected on next use even
  before it expires.
- Impersonation tokens carry a server-side return slot keyed by `jti`;
  `POST /admin/impersonation/end` additionally requires the matching
  browser-bound marker cookie, so a stolen bearer token alone cannot
  redeem the session.

**Residual risk.** Secret compromise. Mitigated operationally by secret
rotation and not logging the secret (see T7).

---

## T3 — RLS bypass leaking cross-tenant data

**Threat.** The admin endpoints intentionally bypass row-level security
(`SET LOCAL row_security = off`) to read across tenants. A bug could
expose that bypass to a non-admin, or an injection could widen a query.

**Mitigations.**
- Only the documented admin registry methods (`*Admin`-suffixed:
  `TenantRegistry.ListAdmin`/`GetAdmin`, `UserRegistry.ListAdminByTenant`,
  `LocationGroupRegistry.ListAdmin`/`GetAdmin`/`MarkPendingDeletionAdmin`,
  `GroupMembershipRegistry.AdminListMembersWithUsers`) issue
  `SET LOCAL row_security = off`, and every caller sits behind
  `RequireSystemAdmin`.
- `SET LOCAL` is transaction-scoped — the bypass cannot leak past the
  request transaction.
- Non-admin endpoints are unchanged: `SET app.current_tenant_id` and the
  RLS policies still scope them per-tenant.
- All admin queries are parameterised (no string-built SQL from user
  input), so a search term cannot widen the row set.

**Residual risk.** A future admin handler that adds a non-`*Admin`
query, or a new `*Admin` method mounted on a route that forgets
`RequireSystemAdmin`. Guarded by the security checklist in the PR and by
the e2e 403 test.

---

## T4 — Impersonation abuse

**Threat.** An operator escalates, pivots, or hides activity through
impersonation.

**Mitigations.**
- Impersonation tokens pin `is_system_admin: false` — an operator
  cannot use an impersonated identity to reach the admin surface.
- **No chaining**: a request already inside an impersonation cannot
  start another (`isImpersonatedRequest` guard + `RequireSystemAdmin`
  rejecting the non-admin impersonation token).
- **No refresh**: `POST /auth/refresh` rejects impersonation tokens
  (bearer `imp=true` and the `imp:<jti>` refresh-cookie marker), so the
  short TTL cannot be extended.
- **No admin targets**: impersonating another system admin is rejected
  (`422 admin.impersonate.target_is_admin`); blocked users are also
  refused.
- **Rate limited**: 10 impersonation starts per operator per hour.
- **Self-block protection**: the block handler resolves the real
  operator via `impersonated_by`, so an operator cannot block their own
  account by acting through an impersonated identity.
- **Auditable**: every action in an impersonated session is logged with
  both the subject (`user_id`) and the operator (`impersonated_by`).
- The UI shows a persistent, non-dismissible banner for the whole
  session.

**Residual risk.** A malicious operator can still view/act on a target's
data within the TTL. This is inherent to a support-impersonation
feature; the audit trail is the compensating control.

---

## T5 — CSRF on admin mutations

**Threat.** A logged-in admin is tricked into issuing a state-changing
admin request from a malicious page.

**Mitigations.**
- All admin mutation endpoints (`POST`/`PATCH`/`DELETE` under
  `/api/v1/admin/*`) run through the existing CSRF middleware and
  require a valid CSRF token.
- CSRF tokens are rotated on impersonation start (to the target) and on
  impersonation end (back to the operator).
- `POST /admin/impersonation/end` is the one mutation mounted outside
  CSRF middleware; it is self-authorising — it requires a validly
  signed impersonation token *and* the matching browser-bound marker
  cookie, which together provide equivalent assurance.

**Residual risk.** Minimal; same posture as the rest of the app.

---

## T6 — Block does not actually cut off access

**Threat.** A blocked user keeps working with already-issued tokens.

**Mitigations.** Block performs a three-part cascade: set
`is_active = false`, revoke all refresh tokens, and bump the
JWT-blacklist `iat`-staleness threshold so live access tokens fail on
the next request. Covered by an integration test and by the e2e spec
(`system-admin.spec.ts`), which asserts that a token issued before the
block returns `401` after it.

**Residual risk.** A window of at most one in-flight request between the
block transaction committing and the next token check.

---

## T7 — Secret / credential leakage via logs

**Threat.** An admin handler logs the JWT secret, a new admin's
password, or an impersonation token to stdout or the structured logger.

**Mitigations.** Admin handlers log identifiers (user/tenant/group IDs,
action names, paths) — not secrets. The CLI prints names/emails only.
Verified by the security checklist below.

**Residual risk.** Regression in a future handler — re-check on review.

---

## Security review checklist (#1758)

Tracked in the PR for #1758; each item is verified against the code
and/or an automated test.

- [ ] **RLS bypass surface** — only the documented `*Admin` registry
      methods issue `SET LOCAL row_security = off`, all behind
      `RequireSystemAdmin`. *(T3)*
- [ ] **JWT claim layout** — `is_system_admin` / `impersonated_by`
      cannot be self-signed by a non-admin (signature verification +
      `userinput:"false"`). *(T1, T2)*
- [ ] **Impersonation no-chain** — e2e + integration test assert nested
      impersonation is rejected. *(T4)*
- [ ] **Impersonation no-refresh** — e2e + integration test assert the
      impersonation token cannot mint a new access token. *(T4)*
- [ ] **Rate-limit on impersonate** — exists (10/operator/hour) and is
      verified by an integration test. *(T4)*
- [ ] **Audit-log coverage** — every admin action writes a row;
      spot-check a real DB row per category. *(audit log)*
- [ ] **CSRF** — admin mutation endpoints require the CSRF token. *(T5)*
- [ ] **Logs** — no admin handler logs the JWT secret, a password, or an
      impersonation token. *(T7)*

---

## See also

- [`devdocs/admin-runbook.md`](../admin-runbook.md) — operator runbook.
- [`devdocs/CSRF_PROTECTION.md`](../CSRF_PROTECTION.md)
- [`devdocs/REFRESH_TOKEN_SYSTEM.md`](../REFRESH_TOKEN_SYSTEM.md)
- [Umbrella #1744](https://github.com/denisvmedia/inventario/issues/1744)
