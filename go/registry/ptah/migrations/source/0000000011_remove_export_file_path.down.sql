-- Rollback removal of file_path column from exports table
-- Add back the file_path column for backward compatibility

-- Add file_path column back
ALTER TABLE exports ADD COLUMN file_path TEXT;
