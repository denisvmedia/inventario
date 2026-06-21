# Admin Section — Threat Model

Focused threat model for the system-wide admin section
([umbrella #1744](https://github.com/denisvmedia/inventario/issues/1744),
QA gate [#1758](https://github.com/denisvmedia/inventario/issues/1758)).
It covers the `/api/v1/admin/*` API subtree, the `/admin/*` UI, the
`system_admin_grants` table (#1784), the advisory `is_system_admin`
JWT claim, and the impersonation primitive.

Scope is deliberately narrow: it addresses what the admin section adds
on top of the existing auth stack (JWT + refresh tokens + CSRF + RLS).
General application threats are out of scope.

## Trust boundaries & assets

- **Assets**: every tenant's data (the admin surface is cross-tenant),
  the `system_admin_grants` table (#1784), the JWT signing secret,
  impersonation tokens, and the audit log.
- **Privileged principals**: users with a row in `system_admin_grants`
  and anyone with database or host shell access (the CLI bootstrap
  path).
- **Boundary**: the `RequireSystemAdmin` middleware separates ordinary
  authenticated users from the admin surface by querying
  `system_admin_grants` on every request; the database role's RLS
  policies separate tenants for all *non-admin* traffic.

---

## T1 — Privilege escalation: a non-admin self-grants admin

**Threat.** A regular user forges or mutates a token / record so that
they reach `/api/v1/admin/*`.

**Mitigations.**
- **Structural (the primary control, #1784):** the system-admin
  privilege is no longer a column on the `users` row. It lives in a
  dedicated `system_admin_grants` table whose only write path is the
  CLI — no HTTP handler can `INSERT`, `UPDATE`, or `DELETE` rows there,
  and no request DTO maps to it. A future handler that does a blind
  decode + Update on `models.User` cannot reach the privilege; the
  worst case is a no-op on a field that no longer exists. This is the
  *structural* control the threat model now relies on.
- Granting / revoking is reachable only via `inventario admin
  grant-system-admin` / `revoke-system-admin`, which require a
  database DSN. The seed reaches the privilege only through the
  `ensureSystemAdminUser` fixture, which is gated behind the
  `INVENTARIO_SEED_SYSTEM_ADMIN_FIXTURE` opt-in (off by default) — so
  the unauthenticated `/api/v1/seed` endpoint cannot mint a
  cross-tenant admin in a production deployment. Defence in depth
  (#2039): the `/api/v1/seed` route itself is now off by default and is
  only mounted when `INVENTARIO_RUN_ENABLE_SEED_ENDPOINT=true`
  (`--enable-seed-endpoint`), set only on the throwaway init-data /
  e2e seed servers — production app servers do not expose it at all,
  so an anonymous caller gets a 404 rather than reaching the
  privileged, RLS-bypassing seed path.
- The JWT `is_system_admin` claim is signed (HS256) with the server
  secret; a tampered claim fails signature verification. The claim is
  an advisory FE hint only — backend authorization always re-queries
  `system_admin_grants` via `RequireSystemAdmin` on every admin
  request (#1784).
- A test invariant (`admin_security_invariants_test.go`) walks every
  registered chi route and asserts no path under `/api/v1/admin/*`
  mounts an HTTP write endpoint for `system_admin_grants`. The
  invariant also asserts that the user-write request DTOs
  (`RegisterRequest`, `UpdateProfileRequest`, …) carry no field that
  could map to a grant write.

**Residual risk.**
- Compromise of the JWT signing secret (see T2) or of the database —
  pre-existing platform-level risks.
- The CLI bootstrap path is the only privileged write surface; whoever
  has shell access to the host (or to a postgres role that can write
  to `system_admin_grants`) can mint admins. Operationally controlled
  by host hardening; not in scope here.

---

## T2 — JWT forgery / replay of admin or impersonation claims

**Threat.** An attacker self-signs a token with `is_system_admin: true`
or `imp: true` / `impersonator_id`, or replays a captured one.

**Mitigations.**
- All tokens are HS256-signed with the server secret and verified on
  every request; the algorithm is pinned (no `alg:none` downgrade).
- Access tokens are short-lived; impersonation tokens are shorter still
  (≤30 min, `INVENTARIO_RUN_IMPERSONATION_TTL`).
- The `is_system_admin` claim is **not** the authorization gate (#1784):
  `RequireSystemAdmin` queries `system_admin_grants` on every admin
  request, so a forged claim that escaped signature verification
  somehow still cannot reach the admin surface — the gate fetches the
  truth fresh from the grant store.
- Block bumps a per-user JWT-blacklist `iat`-staleness threshold, so a
  captured access token for a blocked user is rejected on next use even
  before it expires.
- Impersonation tokens carry a server-side return slot keyed by `jti`;
  `POST /admin/impersonation/end` requires the token's
  `impersonator_id` claim to match the slot's `OperatorUserID` AND the
  slot's `OperatorKind` to be `backoffice_user`. The marker cookie
  binding that previously rode in `imp:<jti>` on the tenant
  `refresh_token` was retired in Phase 5 (issue #1785) — the
  cross-plane redesign issues a tenant impersonation token on behalf
  of a back-office operator, and the tenant cookie path is not used.

**Residual risk.** Secret compromise. Mitigated operationally by secret
rotation and not logging the secret (see T7).

---

## T3 — RLS bypass leaking cross-tenant data

**Threat.** The admin endpoints intentionally bypass row-level security
to read and write across tenants. A bug could expose that bypass to a
non-admin, or an injection could widen a query.

**Mitigations.**
- The bypass is a dedicated Postgres role, not a per-query flag (#1787).
  `inventario_admin` is the *only* role created with the `BYPASSRLS`
  attribute; it has `NOLOGIN`, so nothing connects as it directly.
  Admin registry methods open their transaction through
  `store.DoAsAdmin` / `beginAdminTx`, which issues `SET LOCAL ROLE
  inventario_admin` for the life of that transaction. (The earlier
  `SET LOCAL row_security = off` approach was abandoned: under a
  non-`BYPASSRLS` role Postgres raises `SQLSTATE 42501`, so it 500'd on
  every standard deployment — see the `store.DoAsAdmin` doc comment.)
- Only a small, fixed set of registry methods route through
  `store.DoAsAdmin`: `TenantRegistry.ListAdmin`/`GetAdmin`,
  `UserRegistry.ListAdminByTenant`/`CountSessionsByUser`,
  `LocationGroupRegistry.ListAdmin`/`GetAdmin`/`MarkPendingDeletionAdmin`,
  and `GroupMembershipRegistry.ListByGroupWithUsersAdmin` (surfaced as
  `GroupService.AdminListMembersWithUsers`). Every caller sits behind
  `RequireSystemAdmin`.
- `SET LOCAL ROLE` is transaction-scoped — Postgres resets it on commit
  or rollback, so the bypass cannot leak past the request transaction.
  `BYPASSRLS` lives nowhere but `inventario_admin`: a plain
  `inventario_app` request is still bound by the per-tenant RLS
  policies because it never assumes that role.
- Non-admin endpoints are unchanged: `SET app.current_tenant_id` and the
  RLS policies still scope them per-tenant.
- All admin queries are parameterised (no string-built SQL from user
  input), so a search term cannot widen the row set.
- Regression cover: `go/registry/postgres/admin_rls_bypass_test.go`
  exercises the cross-tenant reads/writes against a real Postgres so a
  future change that drops the `inventario_admin` role or the
  `DoAsAdmin` wrapper fails CI.

**Residual risk.** A future admin handler that adds a query outside
this set, or a new `DoAsAdmin` method mounted on a route that forgets
`RequireSystemAdmin`. Guarded by the security checklist in the PR and by
the e2e 403 test.

---

## T4 — Impersonation abuse

**Threat.** An operator escalates, pivots, or hides activity through
impersonation.

**Mitigations.**
- **Cross-plane authorisation (#1785 Phase 5)**: the start handler
  requires a back-office JWT (`aud=backoffice` + `admin_id`) AND
  `role=platform_admin`. A `support_agent` is refused with `403` and
  `admin.role_required`. A tenant JWT — including one with
  `is_system_admin=true` — is refused at the back-office gate.
- Impersonation tokens pin `is_system_admin: false` — the impersonated
  tenant session cannot reach the back-office admin surface (the
  back-office gate rejects tenant tokens anyway).
- **No chaining**: a request already inside an impersonation cannot
  start another. The impersonation token is a tenant JWT and the
  back-office gate rejects it at the door, so the nested guard in the
  handler is defence-in-depth.
- **No refresh**: `POST /auth/refresh` rejects impersonation tokens
  (`imp=true` bearer check), so the short TTL cannot be extended.
- **No admin targets**: impersonating a tenant user with
  `is_system_admin=true` is rejected (`422
  admin.impersonate.target_is_admin`); blocked users are also refused.
- **Rate limited**: 10 impersonation starts per operator per hour.
- **Self-block protection**: the block handler resolves the real
  operator via `impersonated_by`, so an operator cannot block their own
  account by acting through an impersonated identity.
- **Auditable**: every action in an impersonated session is logged with
  both the subject (`user_id`) and the operator (`impersonated_by` —
  the column name is historical; the wire claim it is populated from
  is `impersonator_id` after Phase 5, with a fallback read of the
  legacy `impersonated_by` claim for in-flight tokens during rolling
  upgrades).
- **End-of-session integrity**: `POST /admin/impersonation/end`
  validates the impersonation token's signature, requires
  `impersonator_id` to match the JTI-keyed return slot's
  `OperatorUserID`, and requires the slot's `OperatorKind` to be
  `backoffice_user`. A forged token with a mismatched operator id is
  refused (`422 admin.impersonate.not_active`).
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
  signed impersonation token (`imp=true`) whose `impersonator_id` matches
  the JTI-keyed server-side return slot (`OperatorUserID`, with
  `OperatorKind == backoffice_user`). The browser-bound `imp:<jti>`
  marker cookie that previously co-authenticated this endpoint was retired
  in #1785 Phase 5; the signed token + return-slot binding now provides the
  assurance on its own.

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

- [ ] **RLS bypass surface** — only the documented fixed set of
      registry methods route through `store.DoAsAdmin` (the
      `inventario_admin` `BYPASSRLS` role), all behind
      `RequireSystemAdmin`. *(T3)*
- [ ] **JWT claim layout** — `is_system_admin` / `impersonated_by`
      cannot be self-signed by a non-admin (signature verification);
      the `is_system_admin` claim is advisory only, with
      `RequireSystemAdmin` re-querying `system_admin_grants` on every
      admin request. *(T1, T2)*
- [ ] **No HTTP write path to system_admin_grants** — the table is
      mutable only via the CLI; `admin_security_invariants_test.go`
      walks every chi route and asserts no admin endpoint can write
      to it. *(T1)*
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
