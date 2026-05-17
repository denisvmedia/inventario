-- Migration rollback
-- Generated on: 2026-05-17T09:19:24Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_maintenance_reminders_group_id;
DROP INDEX IF EXISTS idx_maintenance_reminders_schedule_threshold;
DROP INDEX IF EXISTS idx_maintenance_reminders_tenant_id;
DROP INDEX IF EXISTS idx_maintenance_schedules_commodity;
DROP INDEX IF EXISTS idx_maintenance_schedules_enabled_due;
DROP INDEX IF EXISTS idx_maintenance_schedules_group_due;
DROP INDEX IF EXISTS idx_maintenance_schedules_tenant_group;
DROP INDEX IF EXISTS idx_maintenance_schedules_tenant_id;
DROP INDEX IF EXISTS idx_maintenance_schedules_uuid;
-- Drop RLS policy maintenance_reminder_background_worker_access from table maintenance_reminders
DROP POLICY IF EXISTS maintenance_reminder_background_worker_access ON maintenance_reminders;
-- Drop RLS policy maintenance_reminder_isolation from table maintenance_reminders
DROP POLICY IF EXISTS maintenance_reminder_isolation ON maintenance_reminders;
-- Drop RLS policy maintenance_schedule_background_worker_access from table maintenance_schedules
DROP POLICY IF EXISTS maintenance_schedule_background_worker_access ON maintenance_schedules;
-- Drop RLS policy maintenance_schedule_isolation from table maintenance_schedules
DROP POLICY IF EXISTS maintenance_schedule_isolation ON maintenance_schedules;
-- NOTE: RLS policies were removed from table maintenance_reminders - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table maintenance_schedules - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS maintenance_reminders CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS maintenance_schedules CASCADE;