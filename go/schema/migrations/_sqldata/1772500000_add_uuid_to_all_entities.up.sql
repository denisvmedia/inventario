-- Migration generated from schema differences
-- Generated on: 2026-03-02T00:00:00+00:00
-- Direction: UP

-- Add immutable UUID column to all entity tables.
-- The uuid column is a stable public identifier preserved across export/import/restore cycles.
-- gen_random_uuid()::TEXT auto-populates existing rows.

-- tenants --
ALTER TABLE tenants ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_uuid ON tenants (uuid);

-- users --
ALTER TABLE users ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uuid ON users (uuid);

-- locations --
ALTER TABLE locations ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_locations_uuid ON locations (uuid);

-- areas --
ALTER TABLE areas ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_areas_uuid ON areas (uuid);

-- commodities --
ALTER TABLE commodities ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_commodities_uuid ON commodities (uuid);

-- images --
ALTER TABLE images ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_images_uuid ON images (uuid);

-- invoices --
ALTER TABLE invoices ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_uuid ON invoices (uuid);

-- manuals --
ALTER TABLE manuals ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_manuals_uuid ON manuals (uuid);

-- exports --
ALTER TABLE exports ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_exports_uuid ON exports (uuid);

-- files --
ALTER TABLE files ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_files_uuid ON files (uuid);

-- settings --
ALTER TABLE settings ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_uuid ON settings (uuid);

-- restore_operations --
ALTER TABLE restore_operations ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_restore_operations_uuid ON restore_operations (uuid);

-- restore_steps --
ALTER TABLE restore_steps ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_restore_steps_uuid ON restore_steps (uuid);

-- thumbnail_generation_jobs --
ALTER TABLE thumbnail_generation_jobs ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_thumbnail_jobs_uuid ON thumbnail_generation_jobs (uuid);

-- refresh_tokens --
ALTER TABLE refresh_tokens ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_uuid ON refresh_tokens (uuid);

-- operation_slots --
ALTER TABLE operation_slots ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_slots_uuid ON operation_slots (uuid);

-- audit_logs --
ALTER TABLE audit_logs ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_logs_uuid ON audit_logs (uuid);

-- email_verifications --
ALTER TABLE email_verifications ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_verifications_uuid ON email_verifications (uuid);

-- password_resets --
ALTER TABLE password_resets ADD COLUMN uuid TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_password_resets_uuid ON password_resets (uuid);

