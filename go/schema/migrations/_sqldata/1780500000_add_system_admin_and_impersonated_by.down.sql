-- Migration rollback
-- Generated on: 2026-05-17T20:17:57Z
-- Direction: DOWN

DROP INDEX IF EXISTS users_system_admin_idx;
-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN is_system_admin CASCADE;
-- WARNING: Dropping column users.is_system_admin with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: audit_logs --
-- ALTER statements: --
ALTER TABLE audit_logs DROP COLUMN impersonated_by CASCADE;
-- WARNING: Dropping column audit_logs.impersonated_by with CASCADE - This will delete data and dependent objects! --;