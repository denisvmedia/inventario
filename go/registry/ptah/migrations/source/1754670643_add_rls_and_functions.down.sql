-- Migration rollback
-- Generated on: 2025-08-08T18:30:43+02:00
-- Direction: DOWN

-- Add/modify columns for table: locations --
-- ALTER statements: --
ALTER TABLE locations ADD COLUMN id text PRIMARY KEY NOT NULL;
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN id text PRIMARY KEY NOT NULL;
-- Add/modify columns for table: images --
-- ALTER statements: --
ALTER TABLE images ADD COLUMN id text PRIMARY KEY NOT NULL;
-- Add/modify columns for table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps ADD COLUMN id text PRIMARY KEY NOT NULL;
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ADD COLUMN id text PRIMARY KEY NOT NULL;
-- Add/modify columns for table: areas --
-- ALTER statements: --
ALTER TABLE areas ADD COLUMN id text PRIMARY KEY NOT NULL;
-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ADD COLUMN id text PRIMARY KEY NOT NULL;
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN binary_data_size TYPE bigint;
ALTER TABLE exports ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN binary_data_size SET DEFAULT '''0''::bigint';
-- Modify column exports.binary_data_size: default_expr: 0 -> '0'::bigint --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN file_size TYPE bigint;
ALTER TABLE exports ALTER COLUMN file_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN file_size SET DEFAULT '''0''::bigint';
-- Modify column exports.file_size: default_expr: 0 -> '0'::bigint --
-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ADD COLUMN id text PRIMARY KEY NOT NULL;
-- ALTER statements: --
ALTER TABLE restore_operations ALTER COLUMN binary_data_size TYPE bigint;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size SET DEFAULT '''0''::bigint';
-- Modify column restore_operations.binary_data_size: default_expr: 0 -> '0'::bigint --
-- Add/modify columns for table: invoices --
-- ALTER statements: --
ALTER TABLE invoices ADD COLUMN id text PRIMARY KEY NOT NULL;
-- Add/modify columns for table: manuals --
-- ALTER statements: --
ALTER TABLE manuals ADD COLUMN id text PRIMARY KEY NOT NULL;
DROP INDEX IF EXISTS idx_areas_tenant_id;
DROP INDEX IF EXISTS idx_areas_tenant_location;
DROP INDEX IF EXISTS idx_commodities_tenant_area;
DROP INDEX IF EXISTS idx_commodities_tenant_id;
DROP INDEX IF EXISTS idx_commodities_tenant_status;
DROP INDEX IF EXISTS idx_exports_tenant_id;
DROP INDEX IF EXISTS idx_exports_tenant_status;
DROP INDEX IF EXISTS idx_exports_tenant_type;
DROP INDEX IF EXISTS idx_files_tenant_id;
DROP INDEX IF EXISTS idx_files_tenant_linked_entity;
DROP INDEX IF EXISTS idx_files_tenant_type;
DROP INDEX IF EXISTS idx_images_tenant_commodity;
DROP INDEX IF EXISTS idx_images_tenant_id;
DROP INDEX IF EXISTS idx_invoices_tenant_commodity;
DROP INDEX IF EXISTS idx_invoices_tenant_id;
DROP INDEX IF EXISTS idx_locations_tenant_id;
DROP INDEX IF EXISTS idx_manuals_tenant_commodity;
DROP INDEX IF EXISTS idx_manuals_tenant_id;
DROP INDEX IF EXISTS idx_restore_operations_tenant_export;
DROP INDEX IF EXISTS idx_restore_operations_tenant_id;
DROP INDEX IF EXISTS idx_restore_operations_tenant_status;
DROP INDEX IF EXISTS idx_restore_steps_tenant_id;
DROP INDEX IF EXISTS idx_restore_steps_tenant_operation;
DROP INDEX IF EXISTS idx_restore_steps_tenant_result;
DROP INDEX IF EXISTS tenants_domain_idx;
DROP INDEX IF EXISTS tenants_slug_idx;
DROP INDEX IF EXISTS tenants_status_idx;
DROP INDEX IF EXISTS users_active_idx;
DROP INDEX IF EXISTS users_role_idx;
DROP INDEX IF EXISTS users_tenant_email_idx;
DROP INDEX IF EXISTS users_tenant_idx;
-- Remove columns from table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column locations.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column commodities.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column images.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column restore_steps.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column files.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column areas.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column exports.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column restore_operations.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column invoices.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column manuals.tenant_id with CASCADE - This will delete data and dependent objects! --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS tenants CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS users CASCADE;