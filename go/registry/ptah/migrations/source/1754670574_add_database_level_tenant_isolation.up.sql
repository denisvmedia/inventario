-- Migration generated from schema differences
-- Generated on: 2025-08-08T18:29:34+02:00
-- Direction: UP

-- POSTGRES TABLE: tenants --
CREATE TABLE tenants (
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  domain TEXT,
  name TEXT NOT NULL,
  settings JSONB,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: users --
CREATE TABLE users (
  is_active BOOLEAN NOT NULL DEFAULT 'true',
  email TEXT NOT NULL,
  last_login_at TIMESTAMP,
  updated_at TIMESTAMP NOT NULL,
  password_hash TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  created_at TIMESTAMP NOT NULL,
  tenant_id TEXT NOT NULL,
  CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);
-- Modify table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN id;
-- WARNING: Dropping column commodities.id - This will delete data! --
-- Modify table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ALTER COLUMN binary_data_size TYPE BIGINT;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size SET DEFAULT '0';
-- Modify column restore_operations.binary_data_size: default_expr: '0'::bigint -> 0 --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN id;
-- WARNING: Dropping column restore_operations.id - This will delete data! --
-- Modify table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN id;
-- WARNING: Dropping column images.id - This will delete data! --
-- Modify table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN id;
-- WARNING: Dropping column locations.id - This will delete data! --
-- Modify table: exports --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN file_size TYPE BIGINT;
ALTER TABLE exports ALTER COLUMN file_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN file_size SET DEFAULT '0';
-- Modify column exports.file_size: default_expr: '0'::bigint -> 0 --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN binary_data_size TYPE BIGINT;
ALTER TABLE exports ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN binary_data_size SET DEFAULT '0';
-- Modify column exports.binary_data_size: default_expr: '0'::bigint -> 0 --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN id;
-- WARNING: Dropping column exports.id - This will delete data! --
-- Modify table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN id;
-- WARNING: Dropping column invoices.id - This will delete data! --
-- Modify table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN id;
-- WARNING: Dropping column manuals.id - This will delete data! --
-- Modify table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN id;
-- WARNING: Dropping column restore_steps.id - This will delete data! --
-- Modify table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN id;
-- WARNING: Dropping column files.id - This will delete data! --
-- Modify table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN id;
-- WARNING: Dropping column areas.id - This will delete data! --
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