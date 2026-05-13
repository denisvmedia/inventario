-- Migration rollback
-- Generated on: 2026-05-13T10:37:35Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_group_notification_prefs_tenant_id;
DROP INDEX IF EXISTS idx_group_notification_prefs_unique;
DROP INDEX IF EXISTS idx_group_notification_prefs_user_group;
DROP INDEX IF EXISTS idx_group_notification_prefs_uuid;
-- Drop RLS policy group_notification_prefs_background_worker_access from table group_notification_prefs
DROP POLICY IF EXISTS group_notification_prefs_background_worker_access ON group_notification_prefs;
-- Drop RLS policy group_notification_prefs_tenant_isolation from table group_notification_prefs
DROP POLICY IF EXISTS group_notification_prefs_tenant_isolation ON group_notification_prefs;
-- NOTE: RLS policies were removed from table group_notification_prefs - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS group_notification_prefs CASCADE;