DROP INDEX IF EXISTS idx_exports_deleted_at;

ALTER TABLE exports
DROP COLUMN IF EXISTS deleted_at;
