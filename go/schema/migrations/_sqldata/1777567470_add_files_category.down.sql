-- Migration rollback
-- Generated on: 2026-04-30T18:44:30+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_files_tenant_group_category;
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN category CASCADE;
-- WARNING: Dropping column files.category with CASCADE - This will delete data and dependent objects! --
