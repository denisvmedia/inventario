-- Migration rollback
-- Generated on: 2026-05-08T07:05:03Z
-- Direction: DOWN

-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN size_bytes CASCADE;
-- WARNING: Dropping column files.size_bytes with CASCADE - This will delete data and dependent objects! --;