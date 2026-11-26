-- Migration rollback
-- Generated on: 2026-11-26T16:37:37+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX "idx_intake_sources_failed";
ALTER TABLE "currency_migrations" DROP COLUMN "discarded_at";
ALTER TABLE "exports" DROP COLUMN "attempt_count";
ALTER TABLE "exports" DROP COLUMN "discarded_at";
ALTER TABLE "intake_sources" DROP COLUMN "discarded_at";
ALTER TABLE "restore_operations" DROP COLUMN "discarded_at";
ALTER TABLE "thumbnail_generation_jobs" DROP COLUMN "discarded_at";