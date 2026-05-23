-- Migration rollback: backfill_system_admin_grants
-- Direction: DOWN
--
-- Restore the previous functional state by re-setting
-- `users.is_system_admin = true` for any user that currently has a
-- corresponding row in `system_admin_grants`. Without this, rolling back
-- the data backfill would leave admins effectively de-privileged the
-- moment the schema-drop migration is rolled forward again.
--
-- The DOWN deliberately does NOT delete from `system_admin_grants` — the
-- schema-add migration (#1780600000) owns that table's lifecycle, and a
-- partial rollback should still leave the table intact and re-runnable.

UPDATE users
SET is_system_admin = true,
    updated_at = now()
WHERE id IN (SELECT user_id FROM system_admin_grants);
