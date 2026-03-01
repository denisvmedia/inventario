-- Migration rollback
-- Generated on: 2026-03-01T12:30:07+01:00
-- Direction: DOWN

DROP INDEX IF EXISTS tenants_single_default_idx;
-- Remove columns from table: tenants --
-- ALTER statements: --
ALTER TABLE tenants DROP COLUMN is_default CASCADE;
-- WARNING: Dropping column tenants.is_default with CASCADE - This will delete data and dependent objects! --;