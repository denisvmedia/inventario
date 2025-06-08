-- Rollback migration: convert RFC3339 timestamps back to date-only format

-- Drop indexes first
DROP INDEX IF EXISTS idx_exports_deleted_at;
DROP INDEX IF EXISTS idx_exports_created_date;

-- Add temporary columns with the old date format
ALTER TABLE exports 
ADD COLUMN IF NOT EXISTS created_date_old TEXT,
ADD COLUMN IF NOT EXISTS completed_date_old TEXT,
ADD COLUMN IF NOT EXISTS deleted_at_old TEXT;

-- Convert existing timestamp data back to date format
-- Extract just the date part from RFC3339 format (YYYY-MM-DDTHH:MM:SSZ -> YYYY-MM-DD)
UPDATE exports 
SET created_date_old = CASE 
    WHEN created_date IS NOT NULL AND created_date != '' 
    THEN SUBSTRING(created_date FROM 1 FOR 10)
    ELSE NULL 
END;

UPDATE exports 
SET completed_date_old = CASE 
    WHEN completed_date IS NOT NULL AND completed_date != '' 
    THEN SUBSTRING(completed_date FROM 1 FOR 10)
    ELSE NULL 
END;

UPDATE exports 
SET deleted_at_old = CASE 
    WHEN deleted_at IS NOT NULL AND deleted_at != '' 
    THEN SUBSTRING(deleted_at FROM 1 FOR 10)
    ELSE NULL 
END;

-- Drop the timestamp columns
ALTER TABLE exports 
DROP COLUMN IF EXISTS created_date,
DROP COLUMN IF EXISTS completed_date,
DROP COLUMN IF EXISTS deleted_at;

-- Rename the old columns back to the original names
ALTER TABLE exports 
RENAME COLUMN created_date_old TO created_date;
ALTER TABLE exports 
RENAME COLUMN completed_date_old TO completed_date;
ALTER TABLE exports 
RENAME COLUMN deleted_at_old TO deleted_at;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_exports_created_date ON exports(created_date);
CREATE INDEX IF NOT EXISTS idx_exports_deleted_at ON exports(deleted_at);
