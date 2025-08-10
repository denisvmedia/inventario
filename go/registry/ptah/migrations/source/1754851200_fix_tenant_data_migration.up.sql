-- Migration to properly handle tenant data migration
-- Generated manually on: 2025-08-10T18:40:00Z
-- Direction: UP

-- Step 1: Create tenants table first
CREATE TABLE tenants (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  domain TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Insert default tenant for existing data
INSERT INTO tenants (id, name, slug, status) 
VALUES ('default-tenant', 'Default Tenant', 'default', 'active');

-- Step 3: Add tenant_id columns as nullable first (to allow existing data)
ALTER TABLE areas ADD COLUMN tenant_id TEXT;
ALTER TABLE commodities ADD COLUMN tenant_id TEXT;
ALTER TABLE exports ADD COLUMN tenant_id TEXT;
ALTER TABLE files ADD COLUMN tenant_id TEXT;
ALTER TABLE images ADD COLUMN tenant_id TEXT;
ALTER TABLE invoices ADD COLUMN tenant_id TEXT;
ALTER TABLE locations ADD COLUMN tenant_id TEXT;
ALTER TABLE manuals ADD COLUMN tenant_id TEXT;
ALTER TABLE restore_operations ADD COLUMN tenant_id TEXT;
ALTER TABLE restore_steps ADD COLUMN tenant_id TEXT;

-- Step 4: Update all existing records to use the default tenant
UPDATE areas SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE commodities SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE exports SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE files SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE images SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE invoices SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE locations SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE manuals SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE restore_operations SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;
UPDATE restore_steps SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL;

-- Step 5: Make tenant_id columns NOT NULL now that all records have values
ALTER TABLE areas ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE commodities ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE exports ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE files ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE images ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE invoices ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE locations ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE manuals ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE restore_steps ALTER COLUMN tenant_id SET NOT NULL;

-- Step 6: Add foreign key constraints
ALTER TABLE areas ADD CONSTRAINT fk_areas_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE commodities ADD CONSTRAINT fk_commodities_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE exports ADD CONSTRAINT fk_exports_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE files ADD CONSTRAINT fk_files_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE images ADD CONSTRAINT fk_images_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE invoices ADD CONSTRAINT fk_invoices_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE locations ADD CONSTRAINT fk_locations_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE manuals ADD CONSTRAINT fk_manuals_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE restore_operations ADD CONSTRAINT fk_restore_operations_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE restore_steps ADD CONSTRAINT fk_restore_steps_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);

-- Step 7: Create users table with tenant support
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  email TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_users_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

-- Step 8: Create unique constraint for email per tenant
CREATE UNIQUE INDEX users_tenant_email_idx ON users(tenant_id, email);

-- Step 9: Create performance indexes for tenant-based queries
CREATE INDEX idx_areas_tenant_id ON areas(tenant_id);
CREATE INDEX idx_areas_tenant_location ON areas(tenant_id, location_id);
CREATE INDEX idx_commodities_tenant_id ON commodities(tenant_id);
CREATE INDEX idx_commodities_tenant_area ON commodities(tenant_id, area_id);
CREATE INDEX idx_commodities_tenant_status ON commodities(tenant_id, status);
CREATE INDEX idx_exports_tenant_id ON exports(tenant_id);
CREATE INDEX idx_exports_tenant_status ON exports(tenant_id, status);
CREATE INDEX idx_exports_tenant_type ON exports(tenant_id, type);
CREATE INDEX idx_files_tenant_id ON files(tenant_id);
CREATE INDEX idx_files_tenant_linked_entity ON files(tenant_id, linked_entity_type, linked_entity_id);
CREATE INDEX idx_files_tenant_type ON files(tenant_id, type);
CREATE INDEX idx_images_tenant_id ON images(tenant_id);
CREATE INDEX idx_images_tenant_commodity ON images(tenant_id, commodity_id);
CREATE INDEX idx_invoices_tenant_id ON invoices(tenant_id);
CREATE INDEX idx_invoices_tenant_commodity ON invoices(tenant_id, commodity_id);
CREATE INDEX idx_locations_tenant_id ON locations(tenant_id);
CREATE INDEX idx_manuals_tenant_id ON manuals(tenant_id);
CREATE INDEX idx_manuals_tenant_commodity ON manuals(tenant_id, commodity_id);
CREATE INDEX idx_restore_operations_tenant_id ON restore_operations(tenant_id);
CREATE INDEX idx_restore_operations_tenant_export ON restore_operations(tenant_id, export_id);
CREATE INDEX idx_restore_operations_tenant_status ON restore_operations(tenant_id, status);
CREATE INDEX idx_restore_steps_tenant_id ON restore_steps(tenant_id);
CREATE INDEX idx_restore_steps_tenant_operation ON restore_steps(tenant_id, restore_operation_id);
CREATE INDEX idx_restore_steps_tenant_result ON restore_steps(tenant_id, result);
CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX tenants_slug_idx ON tenants(slug);
CREATE INDEX tenants_domain_idx ON tenants(domain);

-- Step 10: Create application role for Row-Level Security policies
CREATE ROLE inventario_app WITH NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;

-- Step 11: Create custom PostgreSQL functions for tenant context management
CREATE OR REPLACE FUNCTION get_current_tenant_id() RETURNS TEXT AS $$
BEGIN RETURN current_setting('app.current_tenant_id', true); END;
$$
LANGUAGE plpgsql STABLE;

CREATE OR REPLACE FUNCTION set_tenant_context(tenant_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;

-- Step 12: Enable RLS on all tenant-aware tables
ALTER TABLE areas ENABLE ROW LEVEL SECURITY;
ALTER TABLE commodities ENABLE ROW LEVEL SECURITY;
ALTER TABLE exports ENABLE ROW LEVEL SECURITY;
ALTER TABLE files ENABLE ROW LEVEL SECURITY;
ALTER TABLE images ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE locations ENABLE ROW LEVEL SECURITY;
ALTER TABLE manuals ENABLE ROW LEVEL SECURITY;
ALTER TABLE restore_operations ENABLE ROW LEVEL SECURITY;
ALTER TABLE restore_steps ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;

-- Step 13: Create RLS policies for all tenant-aware tables
CREATE POLICY area_tenant_isolation ON areas FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY commodity_tenant_isolation ON commodities FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY export_tenant_isolation ON exports FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY file_tenant_isolation ON files FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY image_tenant_isolation ON images FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY invoice_tenant_isolation ON invoices FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY location_tenant_isolation ON locations FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY manual_tenant_isolation ON manuals FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY restore_operation_tenant_isolation ON restore_operations FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY restore_step_tenant_isolation ON restore_steps FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());

CREATE POLICY user_tenant_isolation ON users FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
