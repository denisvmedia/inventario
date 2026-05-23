-- Migration generated from schema differences
-- Generated on: 2026-05-23T09:05:33Z
-- Direction: UP

DROP INDEX IF EXISTS users_system_admin_idx;
-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN is_system_admin CASCADE;
-- WARNING: Dropping column users.is_system_admin with CASCADE - This will delete data and dependent objects! --;