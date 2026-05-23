# System Admin Operations Runbook

Operational guide for the **system-wide admin section** (umbrella
[#1744](https://github.com/denisvmedia/inventario/issues/1744)). A
*system administrator* is a platform operator with cross-tenant
privileges — distinct from a per-group `admin`/`owner` role. The
privilege lives in the dedicated `system_admin_grants` table (#1784) —
a user is a system admin iff they have a row there — and gates the
`/api/v1/admin/*` API subtree and the `/admin/*` UI.

> **Scope.** System admins can list tenants, inspect/block users,
> oversee groups (including soft-delete), edit group memberships, and
> impersonate users for support. There is **no tenant CRUD** — tenants
> are administered out-of-band (see [CLI Commands](../README.md#cli-commands)).

---

## 1. Granting the first system admin

There is no API to mint the first system admin — that would be a
chicken-and-egg bootstrap. Use the CLI, which talks to the database
directly and needs only a PostgreSQL DSN (no admin session):

```bash
inventario admin grant-system-admin --email admin@acme.com \
  --db-dsn postgres://user:pass@localhost:5432/inventario
```

- The DSN may also come from the `INVENTARIO_DB_DSN` environment
  variable instead of `--db-dsn`.
- **PostgreSQL only.** `memory://` DSNs are rejected — the in-memory
  backend cannot persist the flag across restarts.
- The operation is **idempotent**: granting to a user who is already a
  system admin prints `ℹ️  … is already a system administrator.` and
  exits `0`.
- `grant-system-admin` does **not** add the user to any group — the
  system-admin flag is orthogonal to group membership.

On success:

```
✅ Granted system-admin to Jane Admin (admin@acme.com).
```

Every grant is recorded in the audit log as `admin.grant_system_admin`
(see §4).

---

## 2. Revoking a system admin

```bash
inventario admin revoke-system-admin --email admin@acme.com \
  --db-dsn postgres://user:pass@localhost:5432/inventario
```

- Also **idempotent**: revoking from a non-admin prints
  `ℹ️  … is not a system administrator` and exits `0`.
- **Last-admin guard.** The command refuses to revoke the *last*
  remaining system admin so the platform is never left with zero
  operators. To override this — e.g. a deliberate decommission — pass
  `--allow-zero`:

  ```bash
  inventario admin revoke-system-admin --email admin@acme.com \
    --allow-zero --db-dsn postgres://user:pass@localhost:5432/inventario
  ```

Revocation does **not** terminate the user's live sessions. To also cut
off active access tokens, block the account (admin UI → user detail →
**Block**, or `POST /api/v1/admin/users/{id}/block`), which revokes
refresh tokens and bumps the JWT-blacklist staleness threshold.

To see who currently holds the flag:

```bash
inventario admin list-system-admins \
  --db-dsn postgres://user:pass@localhost:5432/inventario

# machine-readable
inventario admin list-system-admins --output json --db-dsn …
```

---

## 3. Recovery — locked out with no system admins left

If every system admin has been revoked, blocked, or deleted, the
`/api/v1/admin/*` surface becomes unreachable through the application.
**This is fully recoverable** — the CLI is the out-of-band escape hatch
and authenticates against the *database*, not an admin session.

Anyone with shell access to a host that can reach the database can
re-bootstrap:

```bash
inventario admin grant-system-admin --email operator@acme.com \
  --db-dsn postgres://user:pass@localhost:5432/inventario
```

If even that user is blocked, unblock them first:

```bash
inventario users update operator@acme.com --active=true --db-dsn …
inventario admin grant-system-admin --email operator@acme.com --db-dsn …
```

Worst-case fallback — a direct SQL statement (use only if the CLI
binary is unavailable). After #1784 the privilege lives in
`system_admin_grants`, not on `users`:

```sql
INSERT INTO system_admin_grants (id, uuid, user_id, granted_at)
VALUES (
  (gen_random_uuid())::text,
  (gen_random_uuid())::text,
  (SELECT id FROM users WHERE email = 'operator@acme.com'),
  now()
)
ON CONFLICT (user_id) DO NOTHING;
```

Prefer the CLI: it writes an audit-log row and enforces the safety
guards. Treat raw SQL as a break-glass last resort.

---

## 4. Inspecting the audit log

Every admin action — CLI and HTTP — is recorded in the `audit_logs`
table via the `AuditLogRegistry`. There is no dedicated audit-log UI
(only a recent-actions strip on entity pages); inspect the table
directly.

Admin actions use the `admin.` action prefix:

| Action                       | Recorded by                          |
| ----------------------------- | ------------------------------------- |
| `admin.grant_system_admin`    | `inventario admin grant-system-admin` |
| `admin.revoke_system_admin`   | `inventario admin revoke-system-admin`|
| `admin.list_tenants` / `…get_tenant` | tenant list / detail endpoints |
| `admin.list_tenant_users` / `…get_user` | user list / detail endpoints |
| `admin.user_block` / `admin.user_unblock` | block / unblock endpoints |
| `admin.list_groups` / `…get_group` / `…delete_group` | group endpoints |
| `admin.group_member_add` / `…remove` / `…role_change` | membership endpoints |
| `admin.impersonate_start` / `admin.impersonate_end` | impersonation endpoints |

Useful queries:

```sql
-- Recent admin actions
SELECT timestamp, action, user_id, tenant_id, entity_type, entity_id,
       success, impersonated_by
FROM audit_logs
WHERE action LIKE 'admin.%'
ORDER BY timestamp DESC
LIMIT 100;

-- Everything an operator did, including actions taken while impersonating
SELECT timestamp, action, entity_type, entity_id, impersonated_by, success
FROM audit_logs
WHERE user_id = '<operator-user-id>'
   OR impersonated_by = '<operator-user-id>'
ORDER BY timestamp DESC;

-- Every action performed inside any impersonation session
SELECT timestamp, action, user_id AS impersonated_user, impersonated_by AS operator
FROM audit_logs
WHERE impersonated_by IS NOT NULL
ORDER BY timestamp DESC;
```

Key columns: `user_id` is the *acting subject*; `impersonated_by` is the
operator-of-record when the action happened inside an impersonation
session (NULL otherwise); `success` and `error_message` capture failed
attempts. A per-action JSON breadcrumb is stored in `user_agent`.

---

## 5. Impersonation — operational notes

A system admin can issue a short-lived impersonation session for a
target user (admin UI → user detail → **Impersonate**). Operational
constraints:

- Default TTL is **30 minutes**, configurable via
  `INVENTARIO_IMPERSONATION_TTL` (capped at 30 minutes).
- The impersonation token carries `is_system_admin = false` — an
  operator does **not** keep admin powers while impersonating.
- Sessions **cannot** be nested (no impersonating from within an
  impersonation) and **cannot** be refreshed.
- Impersonating another system admin is rejected (`422`,
  `admin.impersonate.target_is_admin`).
- A persistent banner is shown in the UI for the whole session; "End
  impersonation" restores the operator's own session.
- The impersonate endpoint is rate-limited (10 starts per operator per
  hour).

Every action taken during impersonation is audit-logged with both the
subject (`user_id`) and the operator (`impersonated_by`).

---

## See also

- [`devdocs/security/admin-threat-model.md`](security/admin-threat-model.md)
  — threat model for the admin surface.
- [Umbrella #1744](https://github.com/denisvmedia/inventario/issues/1744)
  — design decisions and scope.
- [CLI Commands](../README.md#cli-commands) — tenant / user management.
