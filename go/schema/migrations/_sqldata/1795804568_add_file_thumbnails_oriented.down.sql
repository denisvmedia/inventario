-- Migration rollback
-- Generated on: 2026-11-27T19:36:08+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "files" DROP COLUMN "thumbnails_oriented";