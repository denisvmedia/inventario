-- Migration rollback
-- Generated on: 2026-11-28T13:26:53+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "exports" DROP COLUMN "source_file_name";