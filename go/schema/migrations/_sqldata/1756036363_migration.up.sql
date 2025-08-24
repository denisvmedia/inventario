-- Migration generated from schema differences
-- Generated on: 2025-08-24T13:52:43+02:00
-- Direction: UP

-- Gets the current tenant ID from session for RLS policies
CREATE OR REPLACE FUNCTION get_current_tenant_id() RETURNS TEXT AS $$
BEGIN RETURN current_setting('app.current_tenant_id', true); END;
$$
LANGUAGE plpgsql STABLE;
-- Gets the current user ID from session for RLS policies
CREATE OR REPLACE FUNCTION get_current_user_id() RETURNS TEXT AS $$
BEGIN RETURN current_setting('app.current_user_id', true); END;
$$
LANGUAGE plpgsql STABLE;
-- Sets the current tenant context for RLS policies
CREATE OR REPLACE FUNCTION set_tenant_context(tenant_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;
-- Sets the current user context for RLS policies
CREATE OR REPLACE FUNCTION set_user_context(user_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_user_id', user_id_param, false); END;
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
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: settings --
CREATE TABLE settings (
  name TEXT NOT NULL,
  value JSONB NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: files --
CREATE TABLE files (
  title TEXT,
  description TEXT,
  type TEXT NOT NULL,
  tags JSONB,
  linked_entity_type TEXT,
  linked_entity_id TEXT,
  linked_entity_meta TEXT,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  path TEXT NOT NULL,
  original_path TEXT NOT NULL,
  ext TEXT NOT NULL,
  mime_type TEXT NOT NULL
);
-- POSTGRES TABLE: locations --
CREATE TABLE locations (
  name TEXT NOT NULL,
  address TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: exports --
CREATE TABLE exports (
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  include_file_data BOOLEAN NOT NULL DEFAULT 'false',
  selected_items JSONB,
  file_id TEXT,
  file_path TEXT,
  created_date TIMESTAMP NOT NULL,
  completed_date TIMESTAMP,
  deleted_at TIMESTAMP,
  error_message TEXT,
  description TEXT,
  imported BOOLEAN NOT NULL DEFAULT 'false',
  file_size BIGINT DEFAULT '0',
  location_count INTEGER DEFAULT '0',
  area_count INTEGER DEFAULT '0',
  commodity_count INTEGER DEFAULT '0',
  image_count INTEGER DEFAULT '0',
  invoice_count INTEGER DEFAULT '0',
  manual_count INTEGER DEFAULT '0',
  binary_data_size BIGINT DEFAULT '0',
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: areas --
CREATE TABLE areas (
  name TEXT NOT NULL,
  location_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: restore_operations --
CREATE TABLE restore_operations (
  export_id TEXT NOT NULL,
  description TEXT NOT NULL,
  status TEXT NOT NULL,
  options JSONB NOT NULL,
  created_date TIMESTAMP NOT NULL,
  started_date TIMESTAMP,
  completed_date TIMESTAMP,
  error_message TEXT,
  location_count INTEGER DEFAULT '0',
  area_count INTEGER DEFAULT '0',
  commodity_count INTEGER DEFAULT '0',
  image_count INTEGER DEFAULT '0',
  invoice_count INTEGER DEFAULT '0',
  manual_count INTEGER DEFAULT '0',
  binary_data_size BIGINT DEFAULT '0',
  error_count INTEGER DEFAULT '0',
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: commodities --
CREATE TABLE commodities (
  name TEXT NOT NULL,
  short_name TEXT,
  type TEXT NOT NULL,
  area_id TEXT NOT NULL,
  count INTEGER NOT NULL DEFAULT '1',
  original_price DECIMAL(15,2),
  original_price_currency TEXT,
  converted_original_price DECIMAL(15,2),
  current_price DECIMAL(15,2),
  serial_number TEXT,
  extra_serial_numbers JSONB,
  part_numbers JSONB,
  tags JSONB,
  status TEXT NOT NULL,
  purchase_date TEXT,
  registered_date TEXT,
  last_modified_date TEXT,
  urls JSONB,
  comments TEXT,
  draft BOOLEAN NOT NULL DEFAULT 'false',
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: restore_steps --
CREATE TABLE restore_steps (
  restore_operation_id TEXT NOT NULL,
  name TEXT NOT NULL,
  result TEXT NOT NULL,
  duration BIGINT,
  reason TEXT,
  created_date TIMESTAMP NOT NULL,
  updated_date TIMESTAMP NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: images --
CREATE TABLE images (
  commodity_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  path TEXT NOT NULL,
  original_path TEXT NOT NULL,
  ext TEXT NOT NULL,
  mime_type TEXT NOT NULL
);
-- POSTGRES TABLE: invoices --
CREATE TABLE invoices (
  commodity_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  path TEXT NOT NULL,
  original_path TEXT NOT NULL,
  ext TEXT NOT NULL,
  mime_type TEXT NOT NULL
);
-- POSTGRES TABLE: manuals --
CREATE TABLE manuals (
  commodity_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  path TEXT NOT NULL,
  original_path TEXT NOT NULL,
  ext TEXT NOT NULL,
  mime_type TEXT NOT NULL
);
-- ALTER statements: --
ALTER TABLE users ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE users ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE settings ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE settings ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE files ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE files ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE locations ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE locations ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE exports ADD CONSTRAINT fk_export_file FOREIGN KEY (file_id) REFERENCES files(id);
-- ALTER statements: --
ALTER TABLE exports ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE exports ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE areas ADD CONSTRAINT fk_area_location FOREIGN KEY (location_id) REFERENCES locations(id);
-- ALTER statements: --
ALTER TABLE areas ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE areas ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE restore_operations ADD CONSTRAINT fk_restore_operation_export FOREIGN KEY (export_id) REFERENCES exports(id);
-- ALTER statements: --
ALTER TABLE restore_operations ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE restore_operations ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE commodities ADD CONSTRAINT fk_commodity_area FOREIGN KEY (area_id) REFERENCES areas(id);
-- ALTER statements: --
ALTER TABLE commodities ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE commodities ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE restore_steps ADD CONSTRAINT fk_restore_step_operation FOREIGN KEY (restore_operation_id) REFERENCES restore_operations(id);
-- ALTER statements: --
ALTER TABLE restore_steps ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE restore_steps ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE images ADD CONSTRAINT fk_image_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id);
-- ALTER statements: --
ALTER TABLE images ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE images ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE invoices ADD CONSTRAINT fk_invoice_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id);
-- ALTER statements: --
ALTER TABLE invoices ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE invoices ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE manuals ADD CONSTRAINT fk_manual_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id);
-- ALTER statements: --
ALTER TABLE manuals ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE manuals ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- Enable RLS for images table
ALTER TABLE images ENABLE ROW LEVEL SECURITY;
-- Enable RLS for invoices table
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
-- Enable RLS for locations table
ALTER TABLE locations ENABLE ROW LEVEL SECURITY;
-- Enable RLS for restore_operations table
ALTER TABLE restore_operations ENABLE ROW LEVEL SECURITY;
-- Enable RLS for settings table
ALTER TABLE settings ENABLE ROW LEVEL SECURITY;
-- Enable RLS for users table
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
-- Enable RLS for commodities table
ALTER TABLE commodities ENABLE ROW LEVEL SECURITY;
-- Enable RLS for manuals table
ALTER TABLE manuals ENABLE ROW LEVEL SECURITY;
-- Enable RLS for files table
ALTER TABLE files ENABLE ROW LEVEL SECURITY;
-- Enable RLS for restore_steps table
ALTER TABLE restore_steps ENABLE ROW LEVEL SECURITY;
-- Enable RLS for areas table
ALTER TABLE areas ENABLE ROW LEVEL SECURITY;
-- Enable RLS for exports table
ALTER TABLE exports ENABLE ROW LEVEL SECURITY;
-- Ensures areas can only be accessed by their tenant
DROP POLICY IF EXISTS area_tenant_isolation ON areas;
CREATE POLICY area_tenant_isolation ON areas FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id());
-- Ensures areas can only be accessed and modified by their user
DROP POLICY IF EXISTS area_user_isolation ON areas;
CREATE POLICY area_user_isolation ON areas FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures commodities can only be accessed and modified by their tenant
DROP POLICY IF EXISTS commodity_tenant_isolation ON commodities;
CREATE POLICY commodity_tenant_isolation ON commodities FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures commodities can only be accessed and modified by their user
DROP POLICY IF EXISTS commodity_user_isolation ON commodities;
CREATE POLICY commodity_user_isolation ON commodities FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures exports can only be accessed and modified by their tenant
DROP POLICY IF EXISTS export_tenant_isolation ON exports;
CREATE POLICY export_tenant_isolation ON exports FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures exports can only be accessed and modified by their user
DROP POLICY IF EXISTS export_user_isolation ON exports;
CREATE POLICY export_user_isolation ON exports FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures files can only be accessed and modified by their tenant
DROP POLICY IF EXISTS file_tenant_isolation ON files;
CREATE POLICY file_tenant_isolation ON files FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures files can only be accessed and modified by their user
DROP POLICY IF EXISTS file_user_isolation ON files;
CREATE POLICY file_user_isolation ON files FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures images can only be accessed and modified by their tenant
DROP POLICY IF EXISTS image_tenant_isolation ON images;
CREATE POLICY image_tenant_isolation ON images FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures images can only be accessed and modified by their user
DROP POLICY IF EXISTS image_user_isolation ON images;
CREATE POLICY image_user_isolation ON images FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures invoices can only be accessed and modified by their tenant
DROP POLICY IF EXISTS invoice_tenant_isolation ON invoices;
CREATE POLICY invoice_tenant_isolation ON invoices FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures invoices can only be accessed and modified by their user
DROP POLICY IF EXISTS invoice_user_isolation ON invoices;
CREATE POLICY invoice_user_isolation ON invoices FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures locations can only be accessed by their tenant
DROP POLICY IF EXISTS location_tenant_isolation ON locations;
CREATE POLICY location_tenant_isolation ON locations FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id());
-- Ensures locations can only be accessed and modified by their user
DROP POLICY IF EXISTS location_user_isolation ON locations;
CREATE POLICY location_user_isolation ON locations FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures manuals can only be accessed and modified by their tenant
DROP POLICY IF EXISTS manual_tenant_isolation ON manuals;
CREATE POLICY manual_tenant_isolation ON manuals FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures manuals can only be accessed and modified by their user
DROP POLICY IF EXISTS manual_user_isolation ON manuals;
CREATE POLICY manual_user_isolation ON manuals FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures restore operations can only be accessed and modified by their tenant
DROP POLICY IF EXISTS restore_operation_tenant_isolation ON restore_operations;
CREATE POLICY restore_operation_tenant_isolation ON restore_operations FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures restore operations can only be accessed and modified by their user
DROP POLICY IF EXISTS restore_operation_user_isolation ON restore_operations;
CREATE POLICY restore_operation_user_isolation ON restore_operations FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures restore steps can only be accessed and modified by their tenant
DROP POLICY IF EXISTS restore_step_tenant_isolation ON restore_steps;
CREATE POLICY restore_step_tenant_isolation ON restore_steps FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id())
    WITH CHECK (tenant_id = get_current_tenant_id());
-- Ensures restore steps can only be accessed and modified by their user
DROP POLICY IF EXISTS restore_step_user_isolation ON restore_steps;
CREATE POLICY restore_step_user_isolation ON restore_steps FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures settings can only be accessed by their tenant
DROP POLICY IF EXISTS setting_tenant_isolation ON settings;
CREATE POLICY setting_tenant_isolation ON settings FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id());
-- Ensures settings can only be accessed and modified by their user
DROP POLICY IF EXISTS setting_user_isolation ON settings;
CREATE POLICY setting_user_isolation ON settings FOR ALL TO inventario_app
    USING (user_id = get_current_user_id())
    WITH CHECK (user_id = get_current_user_id());
-- Ensures users can only access their tenant's data
DROP POLICY IF EXISTS user_tenant_isolation ON users;
CREATE POLICY user_tenant_isolation ON users FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id());
-- Ensures users can only access and modify their own data
DROP POLICY IF EXISTS user_user_isolation ON users;
CREATE POLICY user_user_isolation ON users FOR ALL TO inventario_app
    USING (id = get_current_user_id())
    WITH CHECK (id = get_current_user_id());
CREATE INDEX IF NOT EXISTS commodities_active_idx ON commodities (status, area_id) WHERE draft = false;
CREATE INDEX IF NOT EXISTS commodities_draft_idx ON commodities (last_modified_date) WHERE draft = true;
CREATE INDEX IF NOT EXISTS commodities_extra_serial_numbers_gin_idx ON commodities USING GIN (extra_serial_numbers);
CREATE INDEX IF NOT EXISTS commodities_name_trgm_idx ON commodities USING GIN (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS commodities_part_numbers_gin_idx ON commodities USING GIN (part_numbers);
CREATE INDEX IF NOT EXISTS commodities_short_name_trgm_idx ON commodities USING GIN (short_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS commodities_tags_gin_idx ON commodities USING GIN (tags);
CREATE INDEX IF NOT EXISTS commodities_urls_gin_idx ON commodities USING GIN (urls);
CREATE INDEX IF NOT EXISTS files_linked_entity_idx ON files (linked_entity_type, linked_entity_id);
CREATE INDEX IF NOT EXISTS files_linked_entity_meta_idx ON files (linked_entity_type, linked_entity_id, linked_entity_meta);
CREATE INDEX IF NOT EXISTS files_path_trgm_idx ON files USING GIN (path gin_trgm_ops);
CREATE INDEX IF NOT EXISTS files_tags_gin_idx ON files USING GIN (tags);
CREATE INDEX IF NOT EXISTS files_title_trgm_idx ON files USING GIN (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS files_type_created_idx ON files (type, created_at);
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
CREATE INDEX IF NOT EXISTS idx_settings_tenant_id ON settings (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_tenant_user_name ON settings (tenant_id, user_id, name);
CREATE INDEX IF NOT EXISTS idx_settings_user_id ON settings (user_id);
CREATE INDEX IF NOT EXISTS settings_value_gin_idx ON settings USING GIN (value);
CREATE INDEX IF NOT EXISTS tenants_domain_idx ON tenants (domain);
CREATE UNIQUE INDEX IF NOT EXISTS tenants_slug_idx ON tenants (slug);
CREATE INDEX IF NOT EXISTS tenants_status_idx ON tenants (status);
CREATE INDEX IF NOT EXISTS users_active_idx ON users (is_active);
CREATE INDEX IF NOT EXISTS users_role_idx ON users (role);
CREATE UNIQUE INDEX IF NOT EXISTS users_tenant_email_idx ON users (tenant_id, email);
CREATE INDEX IF NOT EXISTS users_tenant_idx ON users (tenant_id);