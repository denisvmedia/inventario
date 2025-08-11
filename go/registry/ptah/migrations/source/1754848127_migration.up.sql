-- Migration generated from schema differences
-- Generated on: 2025-08-10T17:48:47Z
-- Direction: UP

-- Application role for Row-Level Security policies
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'inventario_app') THEN
        CREATE ROLE inventario_app WITH NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
    END IF;
END
$$;

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
-- POSTGRES TABLE: tenants --
CREATE TABLE tenants (
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  domain TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  settings JSONB,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: users --
CREATE TABLE users (
  email TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  is_active BOOLEAN NOT NULL DEFAULT 'true',
  last_login_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  tenant_id TEXT NOT NULL,
  CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);
-- Add/modify columns for table: invoices --
-- ALTER statements: --
ALTER TABLE invoices ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: areas --
-- ALTER statements: --
ALTER TABLE areas ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: locations --
-- ALTER statements: --
ALTER TABLE locations ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: images --
-- ALTER statements: --
ALTER TABLE images ADD COLUMN tenant_id TEXT NOT NULL;
-- Add/modify columns for table: manuals --
-- ALTER statements: --
ALTER TABLE manuals ADD COLUMN tenant_id TEXT NOT NULL;
-- Enable RLS for users table
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
-- Ensures areas can only be accessed by their tenant
DROP POLICY IF EXISTS area_tenant_isolation ON areas;
CREATE POLICY area_tenant_isolation ON areas FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures commodities can only be accessed and modified by their tenant
DROP POLICY IF EXISTS commodity_tenant_isolation ON commodities;
CREATE POLICY commodity_tenant_isolation ON commodities FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures exports can only be accessed and modified by their tenant
DROP POLICY IF EXISTS export_tenant_isolation ON exports;
CREATE POLICY export_tenant_isolation ON exports FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures files can only be accessed and modified by their tenant
DROP POLICY IF EXISTS file_tenant_isolation ON files;
CREATE POLICY file_tenant_isolation ON files FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures images can only be accessed and modified by their tenant
DROP POLICY IF EXISTS image_tenant_isolation ON images;
CREATE POLICY image_tenant_isolation ON images FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures invoices can only be accessed and modified by their tenant
DROP POLICY IF EXISTS invoice_tenant_isolation ON invoices;
CREATE POLICY invoice_tenant_isolation ON invoices FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures locations can only be accessed by their tenant
DROP POLICY IF EXISTS location_tenant_isolation ON locations;
CREATE POLICY location_tenant_isolation ON locations FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures manuals can only be accessed and modified by their tenant
DROP POLICY IF EXISTS manual_tenant_isolation ON manuals;
CREATE POLICY manual_tenant_isolation ON manuals FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures restore operations can only be accessed and modified by their tenant
DROP POLICY IF EXISTS restore_operation_tenant_isolation ON restore_operations;
CREATE POLICY restore_operation_tenant_isolation ON restore_operations FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures restore steps can only be accessed and modified by their tenant
DROP POLICY IF EXISTS restore_step_tenant_isolation ON restore_steps;
CREATE POLICY restore_step_tenant_isolation ON restore_steps FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures users can only access their tenant's data
DROP POLICY IF EXISTS user_tenant_isolation ON users;
CREATE POLICY user_tenant_isolation ON users FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id());
CREATE INDEX IF NOT EXISTS idx_areas_tenant_id ON areas (tenant_id);
CREATE INDEX IF NOT EXISTS idx_areas_tenant_location ON areas (tenant_id, location_id);
CREATE INDEX IF NOT EXISTS idx_commodities_tenant_area ON commodities (tenant_id, area_id);
CREATE INDEX IF NOT EXISTS idx_commodities_tenant_id ON commodities (tenant_id);
CREATE INDEX IF NOT EXISTS idx_commodities_tenant_status ON commodities (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_exports_tenant_id ON exports (tenant_id);
CREATE INDEX IF NOT EXISTS idx_exports_tenant_status ON exports (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_exports_tenant_type ON exports (tenant_id, type);
CREATE INDEX IF NOT EXISTS idx_files_tenant_id ON files (tenant_id);
CREATE INDEX IF NOT EXISTS idx_files_tenant_linked_entity ON files (tenant_id, linked_entity_type, linked_entity_id);
CREATE INDEX IF NOT EXISTS idx_files_tenant_type ON files (tenant_id, type);
CREATE INDEX IF NOT EXISTS idx_images_tenant_commodity ON images (tenant_id, commodity_id);
CREATE INDEX IF NOT EXISTS idx_images_tenant_id ON images (tenant_id);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant_commodity ON invoices (tenant_id, commodity_id);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant_id ON invoices (tenant_id);
CREATE INDEX IF NOT EXISTS idx_locations_tenant_id ON locations (tenant_id);
CREATE INDEX IF NOT EXISTS idx_manuals_tenant_commodity ON manuals (tenant_id, commodity_id);
CREATE INDEX IF NOT EXISTS idx_manuals_tenant_id ON manuals (tenant_id);
CREATE INDEX IF NOT EXISTS idx_restore_operations_tenant_export ON restore_operations (tenant_id, export_id);
CREATE INDEX IF NOT EXISTS idx_restore_operations_tenant_id ON restore_operations (tenant_id);
CREATE INDEX IF NOT EXISTS idx_restore_operations_tenant_status ON restore_operations (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_restore_steps_tenant_id ON restore_steps (tenant_id);
CREATE INDEX IF NOT EXISTS idx_restore_steps_tenant_operation ON restore_steps (tenant_id, restore_operation_id);
CREATE INDEX IF NOT EXISTS idx_restore_steps_tenant_result ON restore_steps (tenant_id, result);
CREATE INDEX IF NOT EXISTS tenants_domain_idx ON tenants (domain);
CREATE UNIQUE INDEX IF NOT EXISTS tenants_slug_idx ON tenants (slug);
CREATE INDEX IF NOT EXISTS tenants_status_idx ON tenants (status);
CREATE INDEX IF NOT EXISTS users_active_idx ON users (is_active);
CREATE INDEX IF NOT EXISTS users_role_idx ON users (role);
CREATE UNIQUE INDEX IF NOT EXISTS users_tenant_email_idx ON users (tenant_id, email);
CREATE INDEX IF NOT EXISTS users_tenant_idx ON users (tenant_id);