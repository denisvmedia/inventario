-- Rollback refactor exports to use file entities
-- Remove file_id field and restore file_path usage

-- Drop the foreign key constraint
ALTER TABLE exports DROP CONSTRAINT IF EXISTS fk_exports_file_id;

-- Drop the index
DROP INDEX IF EXISTS idx_exports_file_id;

-- Remove file_id column
ALTER TABLE exports DROP COLUMN IF EXISTS file_id;
