-- Migration rollback
-- Generated on: 2026-03-02T00:00:00+00:00
-- Direction: DOWN

-- Remove UUID indexes and columns from all entity tables.

-- password_resets --
DROP INDEX IF EXISTS idx_password_resets_uuid;
ALTER TABLE password_resets DROP COLUMN uuid CASCADE;

-- email_verifications --
DROP INDEX IF EXISTS idx_email_verifications_uuid;
ALTER TABLE email_verifications DROP COLUMN uuid CASCADE;

-- audit_logs --
DROP INDEX IF EXISTS idx_audit_logs_uuid;
ALTER TABLE audit_logs DROP COLUMN uuid CASCADE;

-- operation_slots --
DROP INDEX IF EXISTS idx_operation_slots_uuid;
ALTER TABLE operation_slots DROP COLUMN uuid CASCADE;

-- refresh_tokens --
DROP INDEX IF EXISTS idx_refresh_tokens_uuid;
ALTER TABLE refresh_tokens DROP COLUMN uuid CASCADE;

-- thumbnail_generation_jobs --
DROP INDEX IF EXISTS idx_thumbnail_jobs_uuid;
ALTER TABLE thumbnail_generation_jobs DROP COLUMN uuid CASCADE;

-- restore_steps --
DROP INDEX IF EXISTS idx_restore_steps_uuid;
ALTER TABLE restore_steps DROP COLUMN uuid CASCADE;

-- restore_operations --
DROP INDEX IF EXISTS idx_restore_operations_uuid;
ALTER TABLE restore_operations DROP COLUMN uuid CASCADE;

-- settings --
DROP INDEX IF EXISTS idx_settings_uuid;
ALTER TABLE settings DROP COLUMN uuid CASCADE;

-- files --
DROP INDEX IF EXISTS idx_files_uuid;
ALTER TABLE files DROP COLUMN uuid CASCADE;

-- exports --
DROP INDEX IF EXISTS idx_exports_uuid;
ALTER TABLE exports DROP COLUMN uuid CASCADE;

-- manuals --
DROP INDEX IF EXISTS idx_manuals_uuid;
ALTER TABLE manuals DROP COLUMN uuid CASCADE;

-- invoices --
DROP INDEX IF EXISTS idx_invoices_uuid;
ALTER TABLE invoices DROP COLUMN uuid CASCADE;

-- images --
DROP INDEX IF EXISTS idx_images_uuid;
ALTER TABLE images DROP COLUMN uuid CASCADE;

-- commodities --
DROP INDEX IF EXISTS idx_commodities_uuid;
ALTER TABLE commodities DROP COLUMN uuid CASCADE;

-- areas --
DROP INDEX IF EXISTS idx_areas_uuid;
ALTER TABLE areas DROP COLUMN uuid CASCADE;

-- locations --
DROP INDEX IF EXISTS idx_locations_uuid;
ALTER TABLE locations DROP COLUMN uuid CASCADE;

-- users --
DROP INDEX IF EXISTS idx_users_uuid;
ALTER TABLE users DROP COLUMN uuid CASCADE;

-- tenants --
DROP INDEX IF EXISTS idx_tenants_uuid;
ALTER TABLE tenants DROP COLUMN uuid CASCADE;

