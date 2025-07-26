-- Migration generated from schema differences
-- Generated on: 2025-07-26T13:41:10+02:00
-- Direction: UP

-- POSTGRES TABLE: areas --
CREATE TABLE areas (
  name TEXT NOT NULL,
  location_id TEXT NOT NULL,
  CONSTRAINT fk_area_location FOREIGN KEY (location_id) REFERENCES locations(id)
);
-- POSTGRES TABLE: commodities --
CREATE TABLE commodities (
  type TEXT NOT NULL,
  serial_number TEXT,
  short_name TEXT,
  draft BOOLEAN NOT NULL DEFAULT 'false',
  status TEXT NOT NULL,
  count INTEGER NOT NULL DEFAULT '1',
  part_numbers JSONB,
  area_id TEXT NOT NULL,
  purchase_date TEXT,
  registered_date TEXT,
  urls JSONB,
  original_price DECIMAL(15,2),
  extra_serial_numbers JSONB,
  tags JSONB,
  converted_original_price DECIMAL(15,2),
  current_price DECIMAL(15,2),
  name TEXT NOT NULL,
  original_price_currency TEXT,
  last_modified_date TEXT,
  comments TEXT,
  CONSTRAINT fk_commodity_area FOREIGN KEY (area_id) REFERENCES areas(id)
);
-- POSTGRES TABLE: exports --
CREATE TABLE exports (
  description TEXT,
  file_size BIGINT DEFAULT '0',
  completed_date TIMESTAMP,
  error_message TEXT,
  file_path TEXT,
  area_count INTEGER DEFAULT '0',
  created_date TIMESTAMP NOT NULL,
  deleted_at TIMESTAMP,
  type TEXT NOT NULL,
  binary_data_size BIGINT DEFAULT '0',
  status TEXT NOT NULL,
  location_count INTEGER DEFAULT '0',
  invoice_count INTEGER DEFAULT '0',
  include_file_data BOOLEAN NOT NULL DEFAULT 'false',
  selected_items JSONB,
  imported BOOLEAN NOT NULL DEFAULT 'false',
  image_count INTEGER DEFAULT '0',
  file_id TEXT,
  manual_count INTEGER DEFAULT '0',
  commodity_count INTEGER DEFAULT '0',
  CONSTRAINT fk_export_file FOREIGN KEY (file_id) REFERENCES files(id)
);
-- POSTGRES TABLE: files --
CREATE TABLE files (
  created_at TIMESTAMP NOT NULL,
  type TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  linked_entity_meta TEXT,
  linked_entity_type TEXT,
  description TEXT,
  linked_entity_id TEXT,
  tags JSONB,
  title TEXT
);
-- POSTGRES TABLE: images --
CREATE TABLE images (
  commodity_id TEXT NOT NULL,
  CONSTRAINT fk_image_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id)
);
-- POSTGRES TABLE: invoices --
CREATE TABLE invoices (
  commodity_id TEXT NOT NULL,
  CONSTRAINT fk_invoice_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id)
);
-- POSTGRES TABLE: locations --
CREATE TABLE locations (
  name TEXT NOT NULL,
  address TEXT NOT NULL
);
-- POSTGRES TABLE: manuals --
CREATE TABLE manuals (
  commodity_id TEXT NOT NULL,
  CONSTRAINT fk_manual_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id)
);
-- POSTGRES TABLE: settings --
CREATE TABLE settings (
  value JSONB NOT NULL,
  name TEXT PRIMARY KEY NOT NULL
);