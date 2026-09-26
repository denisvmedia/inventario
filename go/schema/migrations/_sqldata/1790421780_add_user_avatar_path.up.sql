-- Migration generated from schema differences
-- Generated on: 2026-09-26T13:23:00+02:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Add/modify columns for table: users --
-- ALTER statements: --
ALTER TABLE "users" ADD COLUMN "avatar_path" TEXT;