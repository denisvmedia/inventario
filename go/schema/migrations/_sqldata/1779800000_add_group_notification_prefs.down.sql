-- Migration rollback: drop the per-group notification prefs table
-- (issue #1648).
-- Direction: DOWN

DROP POLICY IF EXISTS group_notification_prefs_background_worker_access ON group_notification_prefs;
DROP POLICY IF EXISTS group_notification_prefs_tenant_isolation ON group_notification_prefs;

DROP INDEX IF EXISTS idx_group_notification_prefs_tenant_id;
DROP INDEX IF EXISTS idx_group_notification_prefs_user_group;
DROP INDEX IF EXISTS idx_group_notification_prefs_unique;
DROP INDEX IF EXISTS idx_group_notification_prefs_uuid;

DROP TABLE group_notification_prefs CASCADE;
