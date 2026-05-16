-- Migration rollback
-- Generated on: 2026-05-16T16:37:24Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_storage_quota_reminders_group_threshold;
DROP INDEX IF EXISTS idx_storage_quota_reminders_tenant_id;
-- Drop RLS policy storage_quota_reminder_background_worker_access from table storage_quota_reminders
DROP POLICY IF EXISTS storage_quota_reminder_background_worker_access ON storage_quota_reminders;
-- Drop RLS policy storage_quota_reminder_isolation from table storage_quota_reminders
DROP POLICY IF EXISTS storage_quota_reminder_isolation ON storage_quota_reminders;
-- NOTE: RLS policies were removed from table storage_quota_reminders - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS storage_quota_reminders CASCADE;