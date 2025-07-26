ALTER TABLE exports
ADD COLUMN IF NOT EXISTS deleted_at TEXT;

CREATE INDEX IF NOT EXISTS idx_exports_deleted_at ON exports(deleted_at);
