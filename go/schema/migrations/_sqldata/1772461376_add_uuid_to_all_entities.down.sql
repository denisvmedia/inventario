-- Migration rollback
-- Generated on: 2026-03-02T15:22:56+01:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_areas_uuid;
DROP INDEX IF EXISTS idx_audit_logs_uuid;
DROP INDEX IF EXISTS idx_commodities_uuid;
DROP INDEX IF EXISTS idx_email_verifications_uuid;
DROP INDEX IF EXISTS idx_exports_uuid;
DROP INDEX IF EXISTS idx_files_uuid;
DROP INDEX IF EXISTS idx_images_uuid;
DROP INDEX IF EXISTS idx_invoices_uuid;
DROP INDEX IF EXISTS idx_locations_uuid;
DROP INDEX IF EXISTS idx_manuals_uuid;
DROP INDEX IF EXISTS idx_operation_slots_uuid;
DROP INDEX IF EXISTS idx_password_resets_uuid;
DROP INDEX IF EXISTS idx_refresh_tokens_uuid;
DROP INDEX IF EXISTS idx_restore_operations_uuid;
DROP INDEX IF EXISTS idx_restore_steps_uuid;
DROP INDEX IF EXISTS idx_settings_uuid;
DROP INDEX IF EXISTS idx_tenants_uuid;
DROP INDEX IF EXISTS idx_thumbnail_jobs_uuid;
DROP INDEX IF EXISTS idx_users_uuid;
-- Remove columns from table: audit_logs --
-- ALTER statements: --
ALTER TABLE audit_logs DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column audit_logs.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: operation_slots --
-- ALTER statements: --
ALTER TABLE operation_slots DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column operation_slots.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column files.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column areas.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: thumbnail_generation_jobs --
-- ALTER statements: --
ALTER TABLE thumbnail_generation_jobs DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column thumbnail_generation_jobs.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column restore_operations.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column invoices.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: email_verifications --
-- ALTER statements: --
ALTER TABLE email_verifications DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column email_verifications.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column locations.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column commodities.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column images.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column manuals.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: tenants --
-- ALTER statements: --
ALTER TABLE tenants DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column tenants.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: settings --
-- ALTER statements: --
ALTER TABLE settings DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column settings.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column exports.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column restore_steps.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column users.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: refresh_tokens --
-- ALTER statements: --
ALTER TABLE refresh_tokens DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column refresh_tokens.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: password_resets --
-- ALTER statements: --
ALTER TABLE password_resets DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column password_resets.uuid with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: user_concurrency_slots --
-- ALTER statements: --
ALTER TABLE user_concurrency_slots DROP COLUMN uuid CASCADE;
-- WARNING: Dropping column user_concurrency_slots.uuid with CASCADE - This will delete data and dependent objects! --;