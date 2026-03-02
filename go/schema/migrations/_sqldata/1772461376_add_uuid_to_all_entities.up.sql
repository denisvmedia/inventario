-- Migration generated from schema differences
-- Generated on: 2026-03-02T15:22:56+01:00
-- Direction: UP

-- Add/modify columns for table: audit_logs --
-- ALTER statements: --
ALTER TABLE audit_logs ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: operation_slots --
-- ALTER statements: --
ALTER TABLE operation_slots ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: areas --
-- ALTER statements: --
ALTER TABLE areas ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: thumbnail_generation_jobs --
-- ALTER statements: --
ALTER TABLE thumbnail_generation_jobs ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: invoices --
-- ALTER statements: --
ALTER TABLE invoices ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: email_verifications --
-- ALTER statements: --
ALTER TABLE email_verifications ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: locations --
-- ALTER statements: --
ALTER TABLE locations ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: images --
-- ALTER statements: --
ALTER TABLE images ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: manuals --
-- ALTER statements: --
ALTER TABLE manuals ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: tenants --
-- ALTER statements: --
ALTER TABLE tenants ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: settings --
-- ALTER statements: --
ALTER TABLE settings ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: users --
-- ALTER statements: --
ALTER TABLE users ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: refresh_tokens --
-- ALTER statements: --
ALTER TABLE refresh_tokens ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: password_resets --
-- ALTER statements: --
ALTER TABLE password_resets ADD COLUMN uuid TEXT NOT NULL;
-- Add/modify columns for table: user_concurrency_slots --
-- ALTER statements: --
ALTER TABLE user_concurrency_slots ADD COLUMN uuid TEXT NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_areas_uuid ON areas (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_logs_uuid ON audit_logs (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_commodities_uuid ON commodities (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_verifications_uuid ON email_verifications (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_exports_uuid ON exports (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_files_uuid ON files (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_images_uuid ON images (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_uuid ON invoices (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_locations_uuid ON locations (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_manuals_uuid ON manuals (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_slots_uuid ON operation_slots (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_password_resets_uuid ON password_resets (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_uuid ON refresh_tokens (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_restore_operations_uuid ON restore_operations (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_restore_steps_uuid ON restore_steps (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_uuid ON settings (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_uuid ON tenants (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_thumbnail_jobs_uuid ON thumbnail_generation_jobs (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uuid ON users (uuid);