-- Migration rollback
-- Generated on: 2026-05-23T09:05:33Z
-- Direction: DOWN

-- Add/modify columns for table: users --
-- ALTER statements: --
ALTER TABLE users ADD COLUMN is_system_admin boolean NOT NULL DEFAULT 'false';
CREATE INDEX IF NOT EXISTS users_system_admin_idx ON users (is_system_admin) WHERE (is_system_admin = true);