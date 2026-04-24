-- Migration rollback
-- Generated on: 2026-04-24T03:01:06+02:00
-- Direction: DOWN

-- Remove columns from table: tenants --
-- ALTER statements: --
ALTER TABLE tenants DROP COLUMN registration_mode CASCADE;
-- WARNING: Dropping column tenants.registration_mode with CASCADE - This will delete data and dependent objects! --;