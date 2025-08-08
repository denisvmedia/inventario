-- Migration generated from schema differences
-- Generated on: 2025-08-08T18:32:17+02:00
-- Direction: UP

-- Gets the current tenant ID from session for RLS policies
CREATE OR REPLACE FUNCTION get_current_tenant_id() RETURNS TEXT AS $$
BEGIN RETURN current_setting('app.current_tenant_id', true); END;
$$
LANGUAGE plpgsql STABLE;
-- Sets the current tenant context for RLS policies
CREATE OR REPLACE FUNCTION set_tenant_context(tenant_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;
-- POSTGRES TABLE: users --
CREATE TABLE users (
  created_at TIMESTAMP NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  is_active BOOLEAN NOT NULL DEFAULT 'true',
  last_login_at TIMESTAMP,
  email TEXT NOT NULL,
  name TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  tenant_id TEXT NOT NULL,
  CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);
-- POSTGRES TABLE: tenants --
CREATE TABLE tenants (
  domain TEXT,
  updated_at TIMESTAMP NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  created_at TIMESTAMP NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  name TEXT NOT NULL,
  settings JSONB,
  id TEXT PRIMARY KEY NOT NULL
);
-- Add/modify columns for table: locations --
-- Add/modify columns for table: files --
-- Add/modify columns for table: areas --
-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ALTER COLUMN binary_data_size TYPE BIGINT;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size SET DEFAULT '0';
-- Modify column restore_operations.binary_data_size: default_expr: '0'::bigint -> 0 --
-- Add/modify columns for table: images --
-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN binary_data_size TYPE BIGINT;
ALTER TABLE exports ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN binary_data_size SET DEFAULT '0';
-- Modify column exports.binary_data_size: default_expr: '0'::bigint -> 0 --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN file_size TYPE BIGINT;
ALTER TABLE exports ALTER COLUMN file_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN file_size SET DEFAULT '0';
-- Modify column exports.file_size: default_expr: '0'::bigint -> 0 --
-- Add/modify columns for table: commodities --
-- Add/modify columns for table: manuals --
-- Add/modify columns for table: invoices --
-- Add/modify columns for table: restore_steps --
-- Ensures files can only be accessed and modified by their tenant
DROP POLICY IF EXISTS file_tenant_isolation ON files;
CREATE POLICY file_tenant_isolation ON files FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
CREATE INDEX idx_areas_tenant_id ON areas (tenant_id);
CREATE INDEX idx_areas_tenant_location ON areas (tenant_id, location_id);
CREATE INDEX idx_commodities_tenant_area ON commodities (tenant_id, area_id);
CREATE INDEX idx_commodities_tenant_id ON commodities (tenant_id);
CREATE INDEX idx_commodities_tenant_status ON commodities (tenant_id, status);
CREATE INDEX idx_exports_tenant_id ON exports (tenant_id);
CREATE INDEX idx_exports_tenant_status ON exports (tenant_id, status);
CREATE INDEX idx_exports_tenant_type ON exports (tenant_id, type);
CREATE INDEX idx_files_tenant_id ON files (tenant_id);
CREATE INDEX idx_files_tenant_linked_entity ON files (tenant_id, linked_entity_type, linked_entity_id);
CREATE INDEX idx_files_tenant_type ON files (tenant_id, type);
CREATE INDEX idx_images_tenant_commodity ON images (tenant_id, commodity_id);
CREATE INDEX idx_images_tenant_id ON images (tenant_id);
CREATE INDEX idx_invoices_tenant_commodity ON invoices (tenant_id, commodity_id);
CREATE INDEX idx_invoices_tenant_id ON invoices (tenant_id);
CREATE INDEX idx_locations_tenant_id ON locations (tenant_id);
CREATE INDEX idx_manuals_tenant_commodity ON manuals (tenant_id, commodity_id);
CREATE INDEX idx_manuals_tenant_id ON manuals (tenant_id);
CREATE INDEX idx_restore_operations_tenant_export ON restore_operations (tenant_id, export_id);
CREATE INDEX idx_restore_operations_tenant_id ON restore_operations (tenant_id);
CREATE INDEX idx_restore_operations_tenant_status ON restore_operations (tenant_id, status);
CREATE INDEX idx_restore_steps_tenant_id ON restore_steps (tenant_id);
CREATE INDEX idx_restore_steps_tenant_operation ON restore_steps (tenant_id, restore_operation_id);
CREATE INDEX idx_restore_steps_tenant_result ON restore_steps (tenant_id, result);
CREATE INDEX tenants_domain_idx ON tenants (domain);
CREATE UNIQUE INDEX tenants_slug_idx ON tenants (slug);
CREATE INDEX tenants_status_idx ON tenants (status);
CREATE INDEX users_active_idx ON users (is_active);
CREATE INDEX users_role_idx ON users (role);
CREATE UNIQUE INDEX users_tenant_email_idx ON users (tenant_id, email);
CREATE INDEX users_tenant_idx ON users (tenant_id);
-- Remove columns from table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN id CASCADE;
-- WARNING: Dropping column locations.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN id CASCADE;
-- WARNING: Dropping column files.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN id CASCADE;
-- WARNING: Dropping column areas.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN id CASCADE;
-- WARNING: Dropping column restore_operations.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN id CASCADE;
-- WARNING: Dropping column images.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN id CASCADE;
-- WARNING: Dropping column exports.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN id CASCADE;
-- WARNING: Dropping column commodities.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN id CASCADE;
-- WARNING: Dropping column manuals.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN id CASCADE;
-- WARNING: Dropping column invoices.id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN id CASCADE;
-- WARNING: Dropping column restore_steps.id with CASCADE - This will delete data and dependent objects! --;