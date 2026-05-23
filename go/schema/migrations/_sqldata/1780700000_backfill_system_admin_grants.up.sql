-- Migration: backfill_system_admin_grants
-- Direction: UP
--
-- Data backfill (sanctioned hand-written SQL — issue #1784). The migration
-- generator only emits schema-diff SQL; copying existing admin rows from
-- the old `users.is_system_admin` column into the new `system_admin_grants`
-- table needs a hand-written INSERT. The previous migration (#1780600000)
-- added the table; the next migration (#1780800000) will drop the column.
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
