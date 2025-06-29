-- Refactor exports to use file entities instead of custom file handling
-- Add file_id field to exports table and remove file_path

-- Add file_id column to exports table
ALTER TABLE exports ADD COLUMN file_id TEXT;

-- Create foreign key constraint to files table
ALTER TABLE exports ADD CONSTRAINT fk_exports_file_id 
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE SET NULL;

-- Create index for better query performance
CREATE INDEX IF NOT EXISTS idx_exports_file_id ON exports(file_id);

-- Note: We keep file_path for now to allow data migration
-- It will be removed in a subsequent migration after data is migrated
