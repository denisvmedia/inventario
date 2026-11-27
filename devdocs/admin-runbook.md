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
> impersonate users for support. A back-office `platform_admin` also
> creates, edits, suspends, reactivates and deletes tenants (§11); the
> [CLI](../README.md#cli-commands) does the same out-of-band.

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
| `admin.tenant_create` / `…tenant_update` / `…tenant_suspend` / `…tenant_reactivate` / `…tenant_delete` | tenant write endpoints (§11); the breadcrumb holds the slug, the requested name, slug and domain of a create (refused ones included), the reason of a suspend or reactivate, the old and new value of every field an update changes, and the `phase` of a delete (`accepted`, then `finished`) |
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
`orphan-file-gc`, `trash-purge`, `warranty-reminder`,
`storage-quota-reminder`, `loan-reminder`, `maintenance-reminder`,
`item-reminder`, `currency-migration`, `intake`, `weekly-digest`.

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

`orphan-file-gc` and `trash-purge` (§9) are the two destructive periodic
workers in the deployment. `orphan-file-gc` exists as a backstop for residues the delete paths cannot
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
| Files in Trash (`deleted_at` set) | Owned by `trash-purge` (§9). A link to an item in Trash also counts as a live link, so the files of a trashed item are never candidates. |
| `export`-linked files | Owned by the backup subsystem's own lifecycle. The exports table is never probed, so a soft-deleted (recoverable) export can never be misread as "gone". |
| Any unknown / future link type | Fails closed — kept until someone explicitly adds it to the allowlist. |
| `t/<tenant>/files/` blobs | Upload, restore, and `inventario backfill blobs` are all **blob-first / row-second**, so a rowless blob is a normal transient state. Reclaiming these safely needs a provably-complete global keep-set; the failure mode is irreversible loss of the user's file bytes. Out of scope for an autonomous worker. (Crash-window blobs are still reclaimed — via the row sweep, which deletes the blob and thumbnails with the row.) |
| `t/<tenant>/exports/` blobs | A failed export leaves its `.inb` referenced by nothing at all. Fix belongs at the source. |
| `t/<tenant>/restores/` blobs | An uploaded `.inb` awaiting import has **no row of any kind** — and stays rowless forever if the import fails or is never submitted. Deleting one destroys a user's uploaded backup. |
| Seed blobs, legacy flat (pre-#1793) keys | Not under the swept prefix. |

### Rollout: report → delete

The worker ships in **`report` mode and deletes nothing.** A false
positive here is irreversible user data loss (the row delete bypasses Trash
and takes the blob and thumbnails with it), so the
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
| `--orphan-file-gc-interval` | `24h` | Sweep cadence. The worker also sweeps **once at startup**, so a fresh deploy does not wait a full interval for its first report. A restart bypasses nothing: the pause, the in-flight gate and the age gate are all re-evaluated on that first sweep. |

### `inventario_orphan_gc_blocked_tenants` > 0

A tenant is skipped while it has an export or restore in flight — and for
`min_age` after one finishes (a restore writes the *archive's* timestamps
onto the rows it creates, so those rows would otherwise clear the age gate
instantly). The blocking operation is named in an
`event=orphan_gc.blocked` log line.

The gauge reports the **last completed evaluation**, so read it only while
`inventario_orphan_gc_runs_total{result="success"}` is advancing: a tick that
never evaluated the gate (paused worker, `mode=off`, or a gate-build failure)
resets it to `0` rather than leaving the previous tick's value standing and
faking a pinned tenant forever.

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

## 9. Trash purge

Deleting an item or a file moves it to the group's Trash
(`deleted_at` / `deleted_by` set on the row); the blob stays in place and
keeps counting toward the group's storage. A file deleted together with an
item also records `deleted_with_commodity_id`, so restoring the item brings
back exactly those files. Members restore entries from the Trash page; admins
and owners can delete them for good there at any time.

`trash-purge` deletes, across every group, the entries that have been in
Trash longer than the retention. It runs in the `housekeeping` worker group,
sweeps once at startup and then on every interval, in rounds of 100 items
and 100 files until a round finds nothing.

Purging an item deletes its row (the history, loans, services and
maintenance schedules go with it through their foreign keys), removes every
file link to it, and settles every file that went to Trash with it. A purge
never moves a live file to Trash or deletes one: a live file linked to the
item (restored on its own, linked by a merge restore, or missed by the
delete) loses the link and stays, as a standalone file if that was its only
link. The files that went to Trash with the item are settled like this:

- a file also linked to another item in Trash moves to that item;
- a file linked to a live item, location or area comes back as live, and so
  does a file linked to an item the group cannot find;
- anything else is deleted, and its blob is removed only when no other file
  row points at the same blob.

The purge of an item holds the group's currency-migration lock, so it waits
for a running currency migration of that group to finish. So does deleting
an area, which clears the area of the items in Trash that were in it.

Every sweep also settles, whatever their age, the files that went to Trash
with an item that is no longer there: a file whose item is live again comes
back, and a file whose item was purged is settled as above. Such files are
left behind by a purge, restore or delete that stopped halfway; "Empty
trash" settles them for its group as well.

Every step re-reads its state, so a sweep that stops halfway (crash,
deploy, DB error) is finished by the next one. An entry that fails is
skipped and the sweep goes on with the rest; the round logs
`Failed to purge expired Trash entries` with every error, and the sweep
keeps running rounds while they delete something. An entry that keeps
failing is retried on the next interval. A sweep that deleted something
logs `Purged expired Trash entries` with the counts.

### Knobs

| Flag | Env | Default | Notes |
| ---- | --- | ------- | ----- |
| `--trash-retention` | `INVENTARIO_RUN_TRASH_RETENTION` | `720h` (30 days) | How long an entry stays in Trash. The Trash page shows the value in days. |
| `--trash-purge-interval` | `INVENTARIO_RUN_TRASH_PURGE_INTERVAL` | `1h` | Sweep cadence. |

Backups (`.inb` exports) leave Trash out. A full-replace restore empties
the group's Trash first; a merge restore skips any item or file whose id is
in Trash, and creates a file the group does not have, whose only links point
at items in Trash, in Trash with the first of them.

### Stopping it

```bash
inventario workers pause --type trash-purge --reason "holding expired entries" \
  --db-dsn postgres://user:pass@localhost:5432/inventario
```

Entries keep expiring while the worker is paused and are purged on the first
sweep after it resumes.

---

## 10. Failed jobs (#1307)

A background job that fails for good stays in its own table with its last
error, status `failed`. There is no separate dead-letter table: each job
type reads, retries and discards its failed jobs in the table it already
uses.

| Type | Table | Failed when | Retry |
| ---- | ----- | ----------- | ----- |
| `export` | `exports` (`type <> 'imported'`) | the export run returns an error | yes: runs again from the start |
| `import` | `exports` (`type = 'imported'`) | the upload cannot be read or validated | yes: reads the uploaded file again |
| `restore` | `restore_operations` | the restore run returns an error | no: it may have written part of the backup; start a new restore |
| `thumbnail` | `thumbnail_generation_jobs` | the last of its attempts fails | yes: gets a fresh set of attempts |
| `intake` | `intake_sources` | the scan fails for good, or the sweep gives up on it | yes: due at once, with its attempt count kept |
| `currency-migration` | `currency_migrations` | the recovery sweep finds it stuck | no: the rate was confirmed for that moment; start a new migration |
| `personal-data-export` | `personal_data_exports` | the build returns an error, or its worker stops during its third build | yes: builds the copy again from the start |

The email queue, the reminder sweeps, group purge and the cleanup sweepers
keep no failed jobs: they retry on their next run. An export, import or
restore whose process died while it ran stays in progress and never shows
up here ([#2945](https://github.com/denisvmedia/inventario/issues/2945)).

A personal data export belongs to a user, not to a group. Its title is the
user's email address, and it has no group. A build whose worker stops is
taken for dead an hour after it started; the export goes back to the queue,
and once its worker has stopped during its third build it fails with "the
worker stopped before the personal data export finished". Each claim counts
a build in the attempts, a retry's too. A retry clears the error, the
start and end of the build and the archive, and keeps the row for 7 more
days while the copy waits for the worker; when the build finishes, the
copy is kept 7 days from then, like any other. A retry is not a new
request: the user's daily limit still counts from the request they made,
so it neither uses up nor resets their next request.

Only the newest thumbnail job of a file counts. While that job is failed and
nobody has retried or discarded it, viewing the file starts no new job, so a
file that cannot be thumbnailed is one failed job, not one per view. Once it
is discarded, the next view tries again; if that fails too, the new job is
listed.

**Discard** marks a job (`discarded_at`); it does not delete it. The job
leaves the failed list and the depth gauge, but its row and its error stay
until the cleanup of that table removes them (see below). Discarding works
for every type. A retried job leaves the list too; if it fails again, it
comes back. So does an intake scan whose session is resumed, or that is
scanned again from the Inbox: both clear a discard.

### How long a failed job stays

Discarded or not, a failed job stays until its table's own cleanup removes
it:

- `export` and `import`: until the export is deleted, by a member on the
  Exports page or with its group.
- `restore`: until the export it restored from is deleted; deleting an
  export deletes its restores.
- `thumbnail`: the thumbnail worker deletes finished and failed jobs older
  than `--thumbnail-job-retention-period` (24 h by default). After that, the
  next view of the file tries again.
- `intake`: until the member deletes the intake session in the Inbox, or
  the group is deleted.
- `personal-data-export`: until the row expires, 7 days after the user's
  request or after the latest retry; the export worker then deletes it.
  Deleting the user or the tenant deletes it too.
- `currency-migration`: the currency migration worker deletes completed and
  failed migrations, discarded or not, once they ended more than
  `--currency-migration-retention` ago
  (`INVENTARIO_RUN_CURRENCY_MIGRATION_RETENTION`, `2160h` (90 days) by
  default), with their per-item audit rows. It checks once an hour, also
  while the worker is paused. A pending or running migration, and one a
  group still points to (`location_groups.currency_migration_id`), is never
  deleted. The period must be at least `24h`, because the daily limit of two
  migrations per group counts the completed ones since UTC midnight; a
  shorter value stops the server at startup. A deleted migration leaves the
  group's migration history as well; the audit log keeps its
  `currency_migration.start` and `currency_migration.complete` or
  `currency_migration.fail` rows
  ([#2944](https://github.com/denisvmedia/inventario/issues/2944)).

Jobs that failed before the upgrade to this version are listed and counted
like new ones, so an alert on a depth above zero fires from the first
collection until they are retried or discarded one by one
([#2946](https://github.com/denisvmedia/inventario/issues/2946) tracks a
bulk discard).

### Who sees what

- **System admins** see the failed jobs of every tenant on the back office
  page **Admin → Failed jobs** (`/admin/failed-jobs`), with type and tenant
  filters, Retry and Discard. A `support_agent` may list and retry; Discard
  needs the `platform_admin` role.
- **Group admins and owners** see their own group's failed jobs in **Group
  settings → Data & storage**, with Retry. They cannot discard: for a
  restore or a currency migration the card tells them to start a new one,
  and an operator can discard the failed one. Personal data exports are not
  in the group card: they belong to a user, so only the back office and the
  CLI show them, and the group routes answer `failed_job.unknown_type` for
  the type.

### CLI

`inventario jobs failed` acts on every tenant, like the back office page
(PostgreSQL only; `memory://` is rejected):

```bash
# List the failed jobs nobody retried or discarded, newest first
inventario jobs failed list --db-dsn postgres://user:pass@localhost:5432/inventario

# One type, one tenant (id or slug), at most 20 rows (0 prints all)
inventario jobs failed list --type export --tenant acme --limit 20 --db-dsn postgres://...

# Put a job back in its queue
inventario jobs failed retry --type thumbnail <job-id> --db-dsn postgres://...

# Discard a job
inventario jobs failed discard --type restore <job-id> --db-dsn postgres://...
```

`list` prints the type, id, tenant, group, failure time, attempts, whether
the job can be retried, its title and its error. `retry` on a restore or a
currency migration fails with "this kind of job cannot be retried"; a job
that is no longer failed fails with "not found".

### Admin REST API

| Method & path | Effect |
| ------------- | ------ |
| `GET /api/v1/admin/failed-jobs` | List failed jobs across tenants. `?type`, `?tenantID`, `?page`, `?per_page` (default 50, at most 100). `meta.job_types` lists every type with whether it can be retried. |
| `POST /api/v1/admin/failed-jobs/{jobType}/{jobID}/retry` | Retry one job (204). |
| `POST /api/v1/admin/failed-jobs/{jobType}/{jobID}/discard` | Discard one job (204). `platform_admin` only; a `support_agent` gets 403 `admin.role_required`. |
| `GET /api/v1/g/{groupSlug}/failed-jobs` | A group admin's list of the group's failed jobs. Its `meta.job_types` leaves out `personal-data-export`. |
| `POST /api/v1/g/{groupSlug}/failed-jobs/{jobType}/{jobID}/retry` | A group admin's retry. |

Error codes: `failed_job.unknown_type` (404 on a path, 400 on the `?type`
filter), `failed_job.not_found` (404: no such job, or it is no longer
failed), `failed_job.not_retryable` (409). Members without the admin role
get 403 on the group routes.

### Audit

Every retry and discard writes an audit row, also when it is refused, an
unknown job type included: `admin.failed_job_retry` and
`admin.failed_job_discard` from the back office and the CLI (the CLI has no
actor, as for `workers pause`), and `failed_job_retry` from group settings.
The entity type is the job type as sent and the entity id is the job id.
Listing from the back office writes `admin.failed_job_list`, also for a
refused `?type`. A discard that `RequirePlatformAdmin` turns away is not
audited, as for impersonation.

### Observability

- `inventario_jobs_dead_lettered_total{type}` is a counter: the jobs this
  process marked failed for good. It is exported by the processes that run
  workers (`run workers` and `run all`), with a zero series per type from
  the start. `run apiserver` exports no series: the API only queues jobs,
  and the workers that run them are the ones that mark them failed. Sum it
  across processes:
  `sum by (type) (increase(inventario_jobs_dead_lettered_total[1h]))`.
- `inventario_jobs_dead_letter_depth{type}` is a gauge: the failed jobs that
  are neither retried nor discarded, across every tenant. It is collected by
  the housekeeping worker group (or `run all`) on the
  `--business-metrics-interval` cadence, so read it with `max()`, like the
  business gauges. A failed collection keeps the last values and increments
  `inventario_jobs_dead_letter_collect_errors_total`.

---

## 11. Managing tenants (#1037)

A back-office operator with `role=platform_admin` manages tenants in the
back office: **New tenant** on the Tenants page; **Edit** and **Suspend**
or **Reactivate** on a tenant's page; **Delete tenant** in the Danger zone
at the bottom of it. A `support_agent` sees the tenant pages without these
actions, and the API refuses it with `403` and `admin.role_required`. The
endpoints and `inventario tenants` share one set of rules, in
`go/services/admin`.

| Action     | Endpoint                                             | Body or query |
| ---------- | ---------------------------------------------------- | ------------- |
| Create     | `POST /api/v1/admin/tenants`                         | `name`, optional `slug`, `domain`, `status` |
| Edit       | `PATCH /api/v1/admin/tenants/{id}`                   | any of `name`, `slug`, `domain`, `status` |
| Suspend    | `POST /api/v1/admin/tenants/{id}/suspend`            | `reason` |
| Reactivate | `POST /api/v1/admin/tenants/{id}/reactivate`         | `reason` |
| Delete     | `DELETE /api/v1/admin/tenants/{id}?confirm_slug=…`   | the tenant's slug |

Create and edit:

- The name is 1 to 100 characters. An empty slug on create is generated
  from the name, as `inventario tenants create` does: lowercase, every run
  of other characters becomes a hyphen, at most 50 characters. A new
  tenant is active unless `status` says otherwise, and its registration is
  closed.
- Slugs and domains are unique: one another tenant has is refused with
  `409` and `tenant.slug_taken` or `tenant.domain_taken`. A domain is a
  bare lowercase hostname; an empty `domain` on edit removes it, as
  `inventario tenants update <slug> --domain=` does. A unique index on
  `tenants.domain` holds the rule in the database, so two creates or
  edits that give one domain to two tenants at the same time cannot both
  succeed: the second gets the same `409`.
- Tenants written before the index existed (#2948) can share a domain.
  `inventario db migrate up` checks for that before it applies anything,
  and stops with the shared domains and the slugs of their tenants:
  `tenants share a domain, so tenants.domain cannot be made unique:
  "acme.com" (tenants acme, acme-old)`. Leave each domain with one tenant,
  then run the migrations again. `db migrate up --dry-run` runs the same
  check, so it can be run before the upgrade.
- With a tenant base domain (`--tenant-base-domain`), where
  `<slug>.<base domain>` is a tenant's address, a custom domain that is the
  base domain or a host under it is refused with `422` and
  `tenant.domain_reserved`. Hosts are matched against custom domains first,
  so such a domain would take over another tenant's slug address. The CLI
  does not know the base domain and does not apply this check.
- An edit changes only the fields whose value differs from the stored one;
  an edit that changes nothing writes no row and no audit entry. A status
  change away from active ends the tenant's sessions, as a suspension does.

Suspend and reactivate:

- Both take a reason of 1 to 500 characters, which the audit row keeps.
- A suspension ends every session of every user of the tenant, the same
  teardown a user block runs: refresh tokens and personal API tokens are
  revoked, and access tokens issued before the suspension are refused.
- While a tenant is not active, every request on its host is refused with
  `403 Tenant suspended`. Its users get no further on another tenant's
  host: a password or OAuth sign-in finds only users of the host's tenant,
  an API token authenticates only on its own tenant's host, and the MFA
  and magic-link sign-ins, token refresh and access tokens check the
  user's own tenant. Impersonating one of its users is refused with `422`
  and `tenant.not_active`.
- Reactivating makes the tenant active again. The sessions the suspension
  ended stay ended, and the revoked API tokens stay revoked: users sign in
  again and create new tokens.
- A status change made with `inventario tenants update` revokes the
  refresh and API tokens too, but the CLI cannot reach the running
  server's access-token blacklist. An access token issued before such a
  change is refused while the tenant is not active, and works again if the
  tenant is reactivated within the token's 15-minute lifetime.
- The blacklist that refuses access tokens issued before a suspension is
  held in the API process unless `--token-blacklist-redis-url` is set. With
  several replicas, or after the replica that ran the suspension restarts,
  a replica without the entry accepts such a token again once the tenant is
  reactivated within the token's 15 minutes. While the tenant is suspended
  every replica refuses its users. Run more than one replica only with the
  Redis URL set.
- When some sessions cannot be ended, the status change is still saved and
  the API answers `200`; the server logs `Tenant status saved, but ending
  the sessions of some of its users failed` with the tenant, and the audit
  row is a failure with the cause. The tenant's users are refused anyway
  while it is not active. Suspending the tenant again retries the teardown.

Delete:

- The operator types the tenant's slug, the confirmation
  `inventario tenants delete` asks for; the API compares `confirm_slug`
  with the slug exactly and refuses anything else with `422` and
  `admin.tenant.confirmation_mismatch`.
- The request first suspends the tenant and ends its sessions, as Suspend
  does, writes an `admin.tenant_delete` audit row with `phase: accepted`,
  and answers `202` with the suspended tenant. The deletion then runs in
  the background in the API process that took the request, detached from
  the request and bounded at two hours, and writes a second
  `admin.tenant_delete` row with `phase: finished`, a success or a failure
  with the cause. The back office says the deletion has started; the
  tenant shows as suspended until it is gone.
- The order is the CLI's, with a check in front: a dry run of the row
  purge in a transaction that is rolled back, so a foreign key the purge
  does not know stops the deletion before anything is removed; then the
  stored files, while the file rows that tell which files are the tenant's
  are still there; then the users, groups, items and every other row; then
  the tenant itself. It cannot be undone.
- Every step can run again, so a failed deletion, or one cut short when
  the process stopped or restarted, leaves a suspended tenant with part of
  its files and none or all of its rows, and sending the same delete again
  continues it. A delete sent while that process is still deleting the
  tenant answers `202` and starts nothing. The check is per process: a
  delete that reaches another replica starts a second run, and every step
  is safe to repeat, so the run that finds the tenant gone records a
  failure.
- The tenant's audit rows go with it, except the rows of the back office
  (action `admin.*`) and the rows written while an operator impersonated
  one of its users: they record what the operators did, and stay.

The tenant that serves the request's host keeps that host: the API refuses
with `422` and `admin.tenant.serves_this_host` to delete it or to set its
status away from active, because either would lock the back office out.
With a tenant base domain it also refuses to change the domain when the
request came through that domain, and the slug when the request came
through `<slug>.<base domain>` or the catch-all tenant. In single-tenant
mode no host is resolved by a slug, and a host whose domain no tenant has
falls back to the default tenant, so the slug and domain can change there;
the tenant serving the back office is suspended or deleted only with the
CLI, which does not refuse it.

---

## See also

- [`devdocs/security/admin-threat-model.md`](security/admin-threat-model.md)
  — threat model for the admin surface.
- [Umbrella #1744](https://github.com/denisvmedia/inventario/issues/1744)
  — design decisions and scope.
- [CLI Commands](../README.md#cli-commands) — tenant / user management.
