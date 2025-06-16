-- Drop indexes first
DROP INDEX IF EXISTS idx_files_tags;
DROP INDEX IF EXISTS idx_files_created_at;
DROP INDEX IF EXISTS idx_files_type;

-- Drop the table
DROP TABLE IF EXISTS files;
