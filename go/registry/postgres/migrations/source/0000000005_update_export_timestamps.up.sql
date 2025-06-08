-- Update export timestamp fields to use RFC3339 format instead of date-only format
-- This migration converts existing date-only values to proper timestamps

-- For PostgreSQL, we need to handle the conversion carefully
-- since the existing data is in YYYY-MM-DD format and we want RFC3339 format

-- First, add temporary columns with the new format
ALTER TABLE exports 
ADD COLUMN IF NOT EXISTS created_date_new TEXT,
ADD COLUMN IF NOT EXISTS completed_date_new TEXT,
ADD COLUMN IF NOT EXISTS deleted_at_new TEXT;

-- Convert existing data from date format to timestamp format
-- For created_date: convert YYYY-MM-DD to YYYY-MM-DDTHH:MM:SSZ (assuming midnight UTC)
UPDATE exports 
SET created_date_new = CASE 
    WHEN created_date IS NOT NULL AND created_date != '' 
    THEN created_date || 'T00:00:00Z'
    ELSE NULL 
END;

-- For completed_date: convert YYYY-MM-DD to YYYY-MM-DDTHH:MM:SSZ (assuming midnight UTC)
UPDATE exports 
SET completed_date_new = CASE 
    WHEN completed_date IS NOT NULL AND completed_date != '' 
    THEN completed_date || 'T00:00:00Z'
    ELSE NULL 
END;

-- For deleted_at: convert YYYY-MM-DD to YYYY-MM-DDTHH:MM:SSZ (assuming midnight UTC)
UPDATE exports 
SET deleted_at_new = CASE 
    WHEN deleted_at IS NOT NULL AND deleted_at != '' 
    THEN deleted_at || 'T00:00:00Z'
    ELSE NULL 
END;

-- Drop the old columns
ALTER TABLE exports 
DROP COLUMN IF EXISTS created_date,
DROP COLUMN IF EXISTS completed_date,
DROP COLUMN IF EXISTS deleted_at;

-- Rename the new columns to the original names
ALTER TABLE exports 
RENAME COLUMN created_date_new TO created_date;
ALTER TABLE exports 
RENAME COLUMN completed_date_new TO completed_date;
ALTER TABLE exports 
RENAME COLUMN deleted_at_new TO deleted_at;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_exports_created_date ON exports(created_date);
CREATE INDEX IF NOT EXISTS idx_exports_deleted_at ON exports(deleted_at);
