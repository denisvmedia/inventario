-- Migration rollback
-- Generated on: 2026-05-06T19:59:07Z
-- Direction: DOWN

DROP INDEX IF EXISTS commodities_warranty_expires_at_idx;
DROP INDEX IF EXISTS idx_warranty_reminders_commodity_threshold;
DROP INDEX IF EXISTS idx_warranty_reminders_group_id;
DROP INDEX IF EXISTS idx_warranty_reminders_tenant_id;
-- Drop RLS policy warranty_reminder_background_worker_access from table warranty_reminders
DROP POLICY IF EXISTS warranty_reminder_background_worker_access ON warranty_reminders;
-- Drop RLS policy warranty_reminder_isolation from table warranty_reminders
DROP POLICY IF EXISTS warranty_reminder_isolation ON warranty_reminders;
-- NOTE: RLS policies were removed from table warranty_reminders - verify if RLS should be disabled --
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN warranty_expires_at CASCADE;
-- WARNING: Dropping column commodities.warranty_expires_at with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN warranty_notes CASCADE;
-- WARNING: Dropping column commodities.warranty_notes with CASCADE - This will delete data and dependent objects! --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS warranty_reminders CASCADE;