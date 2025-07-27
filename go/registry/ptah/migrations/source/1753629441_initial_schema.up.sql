-- Migration generated from schema differences
-- Generated on: 2025-07-27T17:17:21+02:00
-- Direction: UP

-- Enable GIN indexes on btree types
CREATE EXTENSION IF NOT EXISTS btree_gin;
-- Enable trigram similarity search
CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- POSTGRES TABLE: files --
CREATE TABLE files (
  linked_entity_type TEXT,
  linked_entity_id TEXT,
  description TEXT,
  type TEXT NOT NULL,
  tags JSONB,
  title TEXT,
  linked_entity_meta TEXT,
  updated_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL,
  path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  ext TEXT NOT NULL,
  original_path TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: locations --
CREATE TABLE locations (
  address TEXT NOT NULL,
  name TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: settings --
CREATE TABLE settings (
  name TEXT PRIMARY KEY NOT NULL,
  value JSONB NOT NULL
);
-- POSTGRES TABLE: exports --
CREATE TABLE exports (
  status TEXT NOT NULL,
  file_path TEXT,
  area_count INTEGER DEFAULT '0',
  include_file_data BOOLEAN NOT NULL DEFAULT 'false',
  manual_count INTEGER DEFAULT '0',
  type TEXT NOT NULL,
  commodity_count INTEGER DEFAULT '0',
  invoice_count INTEGER DEFAULT '0',
  deleted_at TIMESTAMP,
  location_count INTEGER DEFAULT '0',
  error_message TEXT,
  created_date TIMESTAMP NOT NULL,
  imported BOOLEAN NOT NULL DEFAULT 'false',
  file_size BIGINT DEFAULT '0',
  binary_data_size BIGINT DEFAULT '0',
  completed_date TIMESTAMP,
  description TEXT,
  image_count INTEGER DEFAULT '0',
  selected_items JSONB,
  file_id TEXT,
  id TEXT PRIMARY KEY NOT NULL,
  CONSTRAINT fk_export_file FOREIGN KEY (file_id) REFERENCES files(id)
);
-- POSTGRES TABLE: areas --
CREATE TABLE areas (
  location_id TEXT NOT NULL,
  name TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  CONSTRAINT fk_area_location FOREIGN KEY (location_id) REFERENCES locations(id)
);
-- POSTGRES TABLE: restore_operations --
CREATE TABLE restore_operations (
  binary_data_size BIGINT DEFAULT '0',
  description TEXT NOT NULL,
  error_message TEXT,
  status TEXT NOT NULL,
  options JSONB NOT NULL,
  invoice_count INTEGER DEFAULT '0',
  commodity_count INTEGER DEFAULT '0',
  created_date TIMESTAMP NOT NULL,
  started_date TIMESTAMP,
  area_count INTEGER DEFAULT '0',
  manual_count INTEGER DEFAULT '0',
  error_count INTEGER DEFAULT '0',
  completed_date TIMESTAMP,
  location_count INTEGER DEFAULT '0',
  export_id TEXT NOT NULL,
  image_count INTEGER DEFAULT '0',
  id TEXT PRIMARY KEY NOT NULL,
  CONSTRAINT fk_restore_operation_export FOREIGN KEY (export_id) REFERENCES exports(id)
);
-- POSTGRES TABLE: commodities --
CREATE TABLE commodities (
  purchase_date TEXT,
  last_modified_date TEXT,
  current_price DECIMAL(15,2),
  original_price DECIMAL(15,2),
  extra_serial_numbers JSONB,
  draft BOOLEAN NOT NULL DEFAULT 'false',
  count INTEGER NOT NULL DEFAULT '1',
  type TEXT NOT NULL,
  part_numbers JSONB,
  urls JSONB,
  short_name TEXT,
  original_price_currency TEXT,
  converted_original_price DECIMAL(15,2),
  tags JSONB,
  name TEXT NOT NULL,
  area_id TEXT NOT NULL,
  serial_number TEXT,
  status TEXT NOT NULL,
  registered_date TEXT,
  comments TEXT,
  id TEXT PRIMARY KEY NOT NULL,
  CONSTRAINT fk_commodity_area FOREIGN KEY (area_id) REFERENCES areas(id)
);
-- POSTGRES TABLE: restore_steps --
CREATE TABLE restore_steps (
  created_date TIMESTAMP NOT NULL,
  name TEXT NOT NULL,
  result TEXT NOT NULL,
  restore_operation_id TEXT NOT NULL,
  reason TEXT,
  updated_date TIMESTAMP NOT NULL,
  duration BIGINT,
  id TEXT PRIMARY KEY NOT NULL,
  CONSTRAINT fk_restore_step_operation FOREIGN KEY (restore_operation_id) REFERENCES restore_operations(id)
);
-- POSTGRES TABLE: images --
CREATE TABLE images (
  commodity_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  ext TEXT NOT NULL,
  original_path TEXT NOT NULL,
  CONSTRAINT fk_image_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id)
);
-- POSTGRES TABLE: invoices --
CREATE TABLE invoices (
  commodity_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  ext TEXT NOT NULL,
  original_path TEXT NOT NULL,
  CONSTRAINT fk_invoice_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id)
);
-- POSTGRES TABLE: manuals --
CREATE TABLE manuals (
  commodity_id TEXT NOT NULL,
  path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  ext TEXT NOT NULL,
  original_path TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  CONSTRAINT fk_manual_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id)
);
CREATE INDEX commodities_active_idx ON commodities (status, area_id) WHERE draft = false;
CREATE INDEX commodities_draft_idx ON commodities (last_modified_date) WHERE draft = true;
CREATE INDEX commodities_extra_serial_numbers_gin_idx ON commodities USING GIN (extra_serial_numbers);
CREATE INDEX commodities_name_trgm_idx ON commodities USING GIN (name gin_trgm_ops);
CREATE INDEX commodities_part_numbers_gin_idx ON commodities USING GIN (part_numbers);
CREATE INDEX commodities_short_name_trgm_idx ON commodities USING GIN (short_name gin_trgm_ops);
CREATE INDEX commodities_tags_gin_idx ON commodities USING GIN (tags);
CREATE INDEX commodities_urls_gin_idx ON commodities USING GIN (urls);
CREATE INDEX files_linked_entity_idx ON files (linked_entity_type, linked_entity_id);
CREATE INDEX files_linked_entity_meta_idx ON files (linked_entity_type, linked_entity_id, linked_entity_meta);
CREATE INDEX files_path_trgm_idx ON files USING GIN (path gin_trgm_ops);
CREATE INDEX files_tags_gin_idx ON files USING GIN (tags);
CREATE INDEX files_title_trgm_idx ON files USING GIN (title gin_trgm_ops);
CREATE INDEX files_type_created_idx ON files (type, created_at);