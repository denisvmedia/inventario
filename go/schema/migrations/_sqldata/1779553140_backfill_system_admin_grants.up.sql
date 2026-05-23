-- Migration: backfill_system_admin_grants
-- Direction: UP
--
-- NOTE: hand-authored under #1784's data-backfill exception. Ptah cannot
-- generate data-only migrations; do not regenerate this file. The project
-- policy is "no hand-written SQL migrations" with the data-backfill flow
-- as its sole sanctioned exception (the user explicitly approved this
-- file). Future readers — human or bot — should leave the file structure
-- intact; only the SQL itself is the editable surface.
--
-- Data backfill (sanctioned hand-written SQL — issue #1784). The migration
-- generator only emits schema-diff SQL; copying existing admin rows from
-- the old `users.is_system_admin` column into the new `system_admin_grants`
-- table needs a hand-written INSERT. The previous migration (#1779553130)
-- added the table; the next migration (#1779553150) will drop the column.
-- This one carries the data across so there's no observable downtime: at
-- every point in the apply sequence at least one source of truth carries
-- the privilege.
--
-- `granted_by` is NULL because the original CLI grant (or seed-data
-- bootstrap) had no recorded operator-of-record on the `users` row.
-- `granted_at` falls back to `users.updated_at` — the closest proxy we
-- have for "when was this flag last touched"; it's not exact but it is
-- monotonic and unambiguous for forensics, and new grants minted after
-- this migration will carry the real CURRENT_TIMESTAMP.
--
-- ON CONFLICT DO NOTHING covers the unique index on user_id: a rerun of
-- the migration (against a partially-migrated database) leaves the
-- already-inserted rows alone instead of failing the whole migration.

INSERT INTO system_admin_grants (id, uuid, user_id, granted_by, granted_at)
SELECT (gen_random_uuid())::text,
       (gen_random_uuid())::text,
       u.id,
       NULL,
       u.updated_at
FROM users u
WHERE u.is_system_admin = true
ON CONFLICT (user_id) DO NOTHING;
