-- Migration rollback
-- Generated on: 2026-11-27T16:00:41+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "personal_data_exports" DROP COLUMN "attempt_count";
ALTER TABLE "personal_data_exports" DROP COLUMN "discarded_at";