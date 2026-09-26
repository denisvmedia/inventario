-- Migration rollback
-- Generated on: 2026-09-26T13:23:00+02:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE "users" DROP COLUMN "avatar_path" CASCADE;
-- WARNING: Dropping column users.avatar_path with CASCADE - This will delete data and dependent objects! --;