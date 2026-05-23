-- Migration generated from schema differences
-- Generated on: 2026-05-17T20:17:57Z
-- Direction: UP

-- Add/modify columns for table: users --
-- ALTER statements: --
ALTER TABLE users ADD COLUMN is_system_admin BOOLEAN NOT NULL DEFAULT 'false';
-- Add/modify columns for table: audit_logs --
-- ALTER statements: --
ALTER TABLE audit_logs ADD COLUMN impersonated_by TEXT;
CREATE INDEX IF NOT EXISTS users_system_admin_idx ON users (is_system_admin) WHERE is_system_admin = true;