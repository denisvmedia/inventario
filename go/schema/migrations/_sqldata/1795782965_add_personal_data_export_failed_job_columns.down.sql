-- Migration rollback
-- Generated on: 2026-11-27T13:36:05+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "personal_data_exports" DROP COLUMN "attempt_count";
ALTER TABLE "personal_data_exports" DROP COLUMN "discarded_at";