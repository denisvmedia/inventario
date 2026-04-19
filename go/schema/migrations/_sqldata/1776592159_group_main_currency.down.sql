-- Migration rollback
-- Generated on: 2026-04-19T11:49:19+02:00
-- Direction: DOWN

-- Remove columns from table: location_groups --
-- ALTER statements: --
ALTER TABLE location_groups DROP COLUMN main_currency CASCADE;
-- WARNING: Dropping column location_groups.main_currency with CASCADE - This will delete data and dependent objects! --;