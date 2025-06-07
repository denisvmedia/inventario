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

CREATE INDEX IF NOT EXISTS idx_exports_status ON exports(status);
CREATE INDEX IF NOT EXISTS idx_exports_created_date ON exports(created_date);
CREATE INDEX IF NOT EXISTS idx_exports_type ON exports(type);
