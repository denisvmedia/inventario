-- Migration rollback: backfill_system_admin_grants
-- Direction: DOWN
--
-- NOTE: hand-authored under #1784's data-backfill exception. Ptah cannot
-- generate data-only migrations; do not regenerate this file. The project
-- policy is "no hand-written SQL migrations" with the data-backfill flow
-- as its sole sanctioned exception (the user explicitly approved this
-- file). Future readers — human or bot — should leave the file structure
-- intact; only the SQL itself is the editable surface.
--
-- Restore the previous functional state by re-setting
-- `users.is_system_admin = true` for any user that currently has a
-- corresponding row in `system_admin_grants`. Without this, rolling back
-- the data backfill would leave admins effectively de-privileged the
-- moment the schema-drop migration is rolled forward again.
--
-- The WHERE clause filters to rows whose flag is not already true so
-- we don't pointlessly bump `updated_at` on users who were already
-- consistent — keeping the rollback's audit footprint to the rows it
-- actually had to change. (Idempotent either way: re-running the DOWN
-- after a no-op rollback touches zero rows.)
--
-- The DOWN deliberately does NOT delete from `system_admin_grants` — the
-- schema-add migration (#1780600000) owns that table's lifecycle, and a
-- partial rollback should still leave the table intact and re-runnable.

UPDATE users
SET is_system_admin = true,
    updated_at = now()
WHERE id IN (SELECT user_id FROM system_admin_grants)
  AND is_system_admin = false;
