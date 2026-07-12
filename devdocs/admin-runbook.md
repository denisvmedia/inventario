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
  backend cannot persist grants across restarts.
- The operation is **idempotent**: granting to a user who is already a
  system admin prints `ℹ️  … is already a system administrator.` and
  exits `0`.
- `grant-system-admin` does **not** add the user to any group — the
  system-admin grant is orthogonal to group membership.

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

To see who currently holds a grant:

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
| `admin.worker_pause` | `POST /api/v1/admin/workers/{type}/pause` (actor = back-office operator) or `inventario workers pause` (`paused_by="cli"`); subject = worker type (#1308) |
| `admin.worker_resume` | `POST /api/v1/admin/workers/{type}/resume` (actor = back-office operator) or `inventario workers resume`; subject = worker type (#1308) |

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

A **back-office operator with `role=platform_admin`** can issue a
short-lived impersonation session for a target tenant user
(admin UI → user detail → **Impersonate**). Phase 5 of issue
[#1785](https://github.com/denisvmedia/inventario/issues/1785) cut the
impersonation surface over from the tenant-side
`users.is_system_admin` gate to the back-office auth plane —
`support_agent` is read-mostly and **cannot** start an impersonation
session. The token still targets a tenant user (so the impersonated
browsing session works against the tenant `/api/v1/g/...` endpoints),
but the operator-of-record is now a `backoffice_users` row.

Operational constraints:

- The caller MUST be a back-office user with `role=platform_admin`.
  A `support_agent` is rejected with `403` and
  `admin.role_required`.
- Default TTL is **30 minutes**, configurable via
  `INVENTARIO_RUN_IMPERSONATION_TTL` (or `--impersonation-ttl`; capped at
  30 minutes).
- The impersonation token carries `is_system_admin = false` and the
  cross-plane operator claims
  `impersonator_id = <backoffice_users.id>` and
  `impersonator_type = "backoffice_user"`. The audit-log
  `impersonated_by` column records the back-office operator id (the
  column name is historical and was kept stable across the rename).
- Sessions **cannot** be nested (no impersonating from within an
  impersonation) and **cannot** be refreshed.
- Impersonating a tenant user whose `is_system_admin = true` is
  rejected (`422`, `admin.impersonate.target_is_admin`); impersonating
  a blocked account is rejected (`422`,
  `admin.impersonate.target_blocked`).
- A persistent banner is shown in the UI for the whole session; "End
  impersonation" restores the operator's back-office session —
  mints a fresh back-office access token + re-plants the
  `backoffice_refresh_token` cookie. The legacy `imp:<jti>` tenant
  marker cookie is gone; the JTI-keyed server-side return slot is
  the only binding between the impersonation token and the operator.
- The impersonate endpoint is rate-limited (10 starts per operator per
  hour).

Every action taken during impersonation is audit-logged with both the
subject (`user_id`) and the operator (`impersonated_by`).

---

## 6. Rolling deploy / rollback — system_admin_grants migration (#1784)

The platform-admin privilege moved from `users.is_system_admin` to the
dedicated `system_admin_grants` table in #1784. The migration ships as
three sequenced steps with strict ordering rules around the application
binary.

### Forward-apply order

1. **Migration A** — schema-add: `CREATE TABLE system_admin_grants`
   (timestamp `1779553130_add_system_admin_grants`). Pure DDL; no app
   change required.
2. **Migration B** — data backfill: copies every
   `users.is_system_admin = true` row into `system_admin_grants` using
   `INSERT ... ON CONFLICT (user_id) DO NOTHING` (timestamp
   `1779553140_backfill_system_admin_grants`). Idempotent — safe to
   re-run.
3. **Application binary** — deploy the new (grants-reading) build.
   `RequireSystemAdmin` and every admin handler now consult
   `SystemAdminGrantRegistry.Exists` instead of the struct field.
4. **Migration C** — schema-drop: removes `users.is_system_admin`
   plus its partial index (timestamp
   `1779553150_drop_users_is_system_admin`).

**Critical ordering**: the new app binary MUST be in place BEFORE
migration C runs. If C lands first, every old-binary instance still
reads `user.IsSystemAdmin` as the zero-value `false` and 403s every
admin user until the rolling deploy completes. A safe rollout pulls
all old replicas before applying C.

### Recovery from a partial migrator run

- **Stops between A and B**: the grants table exists but is empty; the
  old binary keeps working off `users.is_system_admin`. Re-run
  `inventario db migrate up` — the backfill's `ON CONFLICT (user_id)
  DO NOTHING` makes the second pass a no-op for rows that already
  copied. Safe to retry indefinitely.
- **Stops between B and C**: both columns and the grants table are
  populated. Either roll forward (deploy the new binary, apply C) or
  roll back via the down migrations in reverse — there is no
  consistency drift here because the new binary writes to grants AND
  the old binary reads `is_system_admin`. The two sources stay in
  lock-step until C runs.

### Rollback (production safety)

Rollback order is the reverse of forward, with one binary constraint:

1. Apply migration **C-down** — re-adds `users.is_system_admin` with
   default `false` and re-creates the partial index. The column is
   empty; admins read as false on EVERY request until B-down runs.
2. Apply migration **B-down** — re-sets `users.is_system_admin = true`
   for every user with a current `system_admin_grants` row. The
   WHERE clause skips rows that already read true so `updated_at`
   churn is bounded to the rows the rollback actually had to change.
   The grants table itself is NOT dropped; its lifecycle belongs to
   the schema-add migration (A-down).
3. Only AFTER B-down completes is it safe to roll back the
   application binary to a pre-#1784 build. The old binary's
   `RequireSystemAdmin` reads `user.IsSystemAdmin`; rolling it back
   between C-down and B-down would 403 every admin user.
4. (Optional) Apply migration **A-down** to drop the grants table.
   Only do this if you intend to abandon #1784 entirely — leaving the
   table dormant costs nothing and makes a future re-forward
   trivial.

The data-backfill exception (per AGENTS.md) was granted for this
migration set on issue #1784; the SQL was reviewed alongside the
schema migrations.

---

## 7. Pausing / resuming background workers (#1308)

Inventario's polling background workers can be **soft-paused** by a
platform operator
([#1308](https://github.com/denisvmedia/inventario/issues/1308)).
Soft-pause keeps each worker's run loop ticking but makes it **skip its
claim phase** while paused:

- **In-flight jobs finish** — a pause does not interrupt work already
  running; it only stops *new* work from being claimed.
- **No new jobs are claimed** while paused.
- **Resuming is immediate** — it takes effect on the worker's next tick
  (≤ ~10s, the controller poll interval) with **no process restart**.
- **State persists across restarts** and is stored in the
  `worker_control` table. A row with `paused=true` is paused; `paused=false`
  — or no row at all — means the worker runs normally. Resuming flips the
  flag to `false` (the row is kept, not deleted), so a paused-then-resumed
  worker leaves a `paused=false` row behind.
- **The whole fleet is coordinated via the shared database** — every
  replica's pause controller polls the same table, so a single
  pause/resume applies everywhere.

The `worker_control` table is **not tenant-scoped and has no RLS** —
worker pause is a platform-operator control orthogonal to tenants (same
posture as `system_admin_grants` and `audit_logs`).

> **Split deployments (`run workers --workers-only=<group>`):** pause
> state is fleet-wide, but a paused row only has an effect on replicas
> that actually run that worker. The pause controller still polls and
> exposes all canonical types regardless of `--workers-only`, so in a
> process that doesn't schedule a given worker the
> `inventario_worker_paused{type=...}` gauge and `workers status` line
> for it are informational — the worker is paused wherever it *is*
> scheduled (e.g. another group's Deployment).

### Canonical worker types

The pausable worker types (stable identifiers used by the CLI, the API,
and the `worker_control.worker_type` column):

`export`, `import`, `restore`, `thumbnail`, `refresh-token-cleanup`,
`email-verification-cleanup`, `magic-link-token-cleanup`,
`operation-slot-cleanup`, `login-event-retention`, `group-purge`,
`orphan-file-gc`, `warranty-reminder`, `storage-quota-reminder`,
`loan-reminder`, `maintenance-reminder`, `currency-migration`.

> Email delivery is intentionally **not** in this set — it is a Redis
> subscriber rather than a polling worker, with a separate pause story.

### CLI

The `inventario workers` command group mutates pause state directly in
the database (PostgreSQL only — `memory://` is rejected, since the state
must persist in a database shared with the worker process):

```bash
# Pause a worker (optionally with a reason)
inventario workers pause --type export --reason "maintenance window" \
  --db-dsn postgres://user:pass@localhost:5432/inventario

# Resume it
inventario workers resume --type export --db-dsn postgres://...

# Show the pause state of every worker (no row => running)
inventario workers status --db-dsn postgres://...
```

> This is **not** the same as `inventario run workers`, which *starts*
> the worker process. The `workers` group mutates the pause state of an
> already-running deployment.

A CLI pause records `paused_by = "cli"` (the CLI has no authenticated
operator session). Each operation writes an audit row
(`admin.worker_pause` / `admin.worker_resume`, see §4) with the worker
type as the subject.

### Admin REST API

The same control is exposed under the back-office-gated admin subtree
(requires a back-office session, `RequireBackofficeAuth`):

| Method & path | Effect |
| ------------- | ------ |
| `GET /api/v1/admin/workers` | List every canonical worker type with its pause state (`type: "worker_control"`, `id`: the worker type). |
| `POST /api/v1/admin/workers/{workerType}/pause` | Soft-pause the worker. Optional `{"reason": "..."}` body (≤ 500 chars; empty body allowed). |
| `POST /api/v1/admin/workers/{workerType}/resume` | Resume the worker (no body). |

- An unknown `{workerType}` returns **404** with code
  `admin.worker.unknown_type`; a reason over 500 characters returns
  **422** with `admin.worker.reason_too_long`.
- A pause via the API records `paused_by = <backoffice_users.id>` (the
  operator of record) rather than the CLI's `"cli"` marker.

### Observability

- **Metric:** `inventario_worker_paused{type="<worker>"}` is a gauge —
  `1` while the named worker is soft-paused, `0` otherwise. The series
  exists for every canonical type (initialised to `0` at startup) so
  dashboards and alerts always have the full label set.
- **Log lines (once per transition):** `worker paused`
  (`type`, and `reason` / `paused_by` when present) and
  `worker resumed` (`type`). These are emitted by the pause controller
  on the paused↔running flip, not on every poll.
- **Fail-safe:** if the controller's poll of `worker_control` fails, the
  last-known state is **retained** (a DB blip cannot silently un-pause a
  worker an operator deliberately stopped); the failure is logged once
  on the error transition.

---

## 8. Orphan-file GC (#2237)

`orphan-file-gc` is the **only destructive periodic worker** in the
deployment. It exists as a backstop for residues the delete paths cannot
close by construction (they are row-first / blob-best-effort and
multi-transaction, because the file→entity link is polymorphic and cannot
carry an FK):

1. **Crash window** — a process dies between an entity-row delete and its
   linked-file sweep, leaving file rows pointing at an entity that no
   longer exists.
2. **Concurrent attach** — a file is linked to an entity while that
   entity's delete is in flight.
3. **Thumbnail mid-generation race** — a file is deleted while the
   thumbnail worker sits between its read and its blob writes.

### What it sweeps — and what it will NEVER touch

It sweeps exactly two classes:

- **File ROWS** whose `linked_entity_type` is one of
  `commodity` / `area` / `location` **and** whose `linked_entity_id`
  names a row that does not exist anywhere in the database.
- **THUMBNAIL blobs** under `t/<tenant>/thumbnails/` whose owning file
  row is gone. Thumbnails are derived and regenerable, and the key
  embeds the owning row's primary key, so orphan-ness is an exact
  single-row question.

Everything else is **NEVER-SWEEP** — not filtered out, but never
enumerated in the first place:

| Class | Why it is never touched |
| ----- | ----------------------- |
| Standalone files (`linked_entity_type = ''`) | First-class since #2235; exported in backups. **"No link" is not "orphan."** The candidate predicate is a positive allowlist, so a standalone file cannot enter it. |
| `export`-linked files | Owned by the backup subsystem's own lifecycle. The exports table is never probed, so a soft-deleted (recoverable) export can never be misread as "gone". |
| Any unknown / future link type | Fails closed — kept until someone explicitly adds it to the allowlist. |
| `t/<tenant>/files/` blobs | Upload, restore, and `inventario backfill blobs` are all **blob-first / row-second**, so a rowless blob is a normal transient state. Reclaiming these safely needs a provably-complete global keep-set; the failure mode is irreversible loss of the user's file bytes. Out of scope for an autonomous worker. (Crash-window blobs are still reclaimed — via the row sweep, which deletes the blob and thumbnails with the row.) |
| `t/<tenant>/exports/` blobs | A failed export leaves its `.inb` referenced by nothing at all. Fix belongs at the source. |
| `t/<tenant>/restores/` blobs | An uploaded `.inb` awaiting import has **no row of any kind** — and stays rowless forever if the import fails or is never submitted. Deleting one destroys a user's uploaded backup. |
| Seed blobs, legacy flat (pre-#1793) keys | Not under the swept prefix. |

### Rollout: report → delete

The worker ships in **`report` mode and deletes nothing.** A false
positive here is irreversible user data loss (`files` has no soft-delete
column, and the row delete takes the blob and thumbnails with it), so the
predicate has to be observed against real data before it earns the right
to delete.

```bash
# 1. Default. Scan + log + metrics, DELETE NOTHING.
--orphan-file-gc-mode=report

# 2. Only after watching the candidate stream for a full release cycle.
--orphan-file-gc-mode=delete

# Or skip the scan entirely (no bucket LIST cost).
--orphan-file-gc-mode=off
```

Watch **`inventario_orphan_gc_candidates_total{kind="row"|"thumbnail"}`**
— it increments in *both* modes. Steady state should be ~0; a small,
explainable number after a known crash is expected. Every candidate is
logged (`event=orphan_gc.row` / `event=orphan_gc.thumbnail`) with enough
detail to hand-verify it against the UI, and — in delete mode — the log
line is the only artifact from which a destroyed file can be
reconstructed.

### Knobs

| Flag | Default | Notes |
| ---- | ------- | ----- |
| `--orphan-file-gc-mode` | `report` | `off` \| `report` \| `delete`. An unknown value **fails startup**. |
| `--orphan-file-gc-min-age` | `72h` | Minimum age of a row (both `created_at` *and* `updated_at`) or a blob (bucket `ModTime`). **Hard floor 24h** — a lower value fails startup. |
| `--orphan-file-gc-interval` | `24h` | Sweep cadence. |

### `inventario_orphan_gc_blocked_tenants` > 0

A tenant is skipped while it has an export or restore in flight — and for
`min_age` after one finishes (a restore writes the *archive's* timestamps
onto the rows it creates, so those rows would otherwise clear the age gate
instantly). The blocking operation is named in an
`event=orphan_gc.blocked` log line.

There is no heartbeat on exports/restores, so a **crashed** operation stays
`running` / `in_progress` forever and pins its tenant permanently. That is
the correct direction (under-collecting forever beats one wrong delete),
but it needs manual attention: investigate and resolve the stuck operation.
**Do not add a "stale after N hours → ignore it" escape hatch** — that
re-opens exactly the race the gate closes.

### Stopping it

```bash
inventario workers pause --type orphan-file-gc --reason "investigating a candidate" \
  --db-dsn postgres://user:pass@localhost:5432/inventario
```

The sweep is skipped on the next tick; no restart needed.

---

## See also

- [`devdocs/security/admin-threat-model.md`](security/admin-threat-model.md)
  — threat model for the admin surface.
- [Umbrella #1744](https://github.com/denisvmedia/inventario/issues/1744)
  — design decisions and scope.
- [CLI Commands](../README.md#cli-commands) — tenant / user management.
