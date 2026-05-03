-- Migration rollback
-- Generated on: 2026-05-03T15:46:26+02:00
-- Direction: DOWN

-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN file_count CASCADE;
-- WARNING: Dropping column exports.file_count with CASCADE - This will delete data and dependent objects! --

-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN file_count CASCADE;
-- WARNING: Dropping column restore_operations.file_count with CASCADE - This will delete data and dependent objects! --
