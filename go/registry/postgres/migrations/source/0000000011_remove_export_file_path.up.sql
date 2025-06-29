-- Remove deprecated file_path column from exports table
-- This migration should be run after all exports have been migrated to use file entities

-- Remove the deprecated file_path column
ALTER TABLE exports DROP COLUMN IF EXISTS file_path;
