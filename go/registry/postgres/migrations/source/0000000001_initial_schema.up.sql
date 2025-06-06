CREATE TABLE IF NOT EXISTS locations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    address TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS areas (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS commodities (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    short_name TEXT,
    type TEXT NOT NULL,
    area_id TEXT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    count INTEGER NOT NULL DEFAULT 1,
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
    draft BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS images (
    id TEXT PRIMARY KEY,
    commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    original_path TEXT NOT NULL,
    ext TEXT NOT NULL,
    mime_type TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY,
    commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    original_path TEXT NOT NULL,
    ext TEXT NOT NULL,
    mime_type TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS manuals (
    id TEXT PRIMARY KEY,
    commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    original_path TEXT NOT NULL,
    ext TEXT NOT NULL,
    mime_type TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS exports (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    include_file_data BOOLEAN NOT NULL DEFAULT FALSE,
    selected_items JSONB,
    file_path TEXT,
    created_date TEXT NOT NULL,
    completed_date TEXT,
    error_message TEXT,
    description TEXT
);

CREATE TABLE IF NOT EXISTS settings (
    name TEXT PRIMARY KEY,
    value JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_exports_status ON exports(status);
CREATE INDEX IF NOT EXISTS idx_exports_created_date ON exports(created_date);
CREATE INDEX IF NOT EXISTS idx_exports_type ON exports(type);
