-- Create restore_operations table
CREATE TABLE IF NOT EXISTS restore_operations (
    id TEXT PRIMARY KEY,
    export_id TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL,
    options TEXT NOT NULL,
    created_date TEXT NOT NULL,
    started_date TEXT,
    completed_date TEXT,
    error_message TEXT,
    location_count INTEGER DEFAULT 0,
    area_count INTEGER DEFAULT 0,
    commodity_count INTEGER DEFAULT 0,
    image_count INTEGER DEFAULT 0,
    invoice_count INTEGER DEFAULT 0,
    manual_count INTEGER DEFAULT 0,
    binary_data_size BIGINT DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    FOREIGN KEY (export_id) REFERENCES exports(id) ON DELETE CASCADE
);

-- Create restore_steps table
CREATE TABLE IF NOT EXISTS restore_steps (
    id TEXT PRIMARY KEY,
    restore_operation_id TEXT NOT NULL,
    name TEXT NOT NULL,
    result TEXT NOT NULL,
    duration BIGINT,
    reason TEXT,
    created_date TEXT NOT NULL,
    updated_date TEXT NOT NULL,
    FOREIGN KEY (restore_operation_id) REFERENCES restore_operations(id) ON DELETE CASCADE
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_restore_operations_export_id ON restore_operations(export_id);
CREATE INDEX IF NOT EXISTS idx_restore_operations_status ON restore_operations(status);
CREATE INDEX IF NOT EXISTS idx_restore_operations_created_date ON restore_operations(created_date);

CREATE INDEX IF NOT EXISTS idx_restore_steps_operation_id ON restore_steps(restore_operation_id);
CREATE INDEX IF NOT EXISTS idx_restore_steps_result ON restore_steps(result);
CREATE INDEX IF NOT EXISTS idx_restore_steps_created_date ON restore_steps(created_date);
