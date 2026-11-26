-- Migration generated from schema differences
-- Generated on: 2026-11-26T13:54:56+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "currency_migrations" ADD COLUMN "discarded_at" TIMESTAMP;
ALTER TABLE "exports" ADD COLUMN "attempt_count" INTEGER NOT NULL DEFAULT 0;
ALTER TABLE "exports" ADD COLUMN "discarded_at" TIMESTAMP;
ALTER TABLE "intake_sources" ADD COLUMN "discarded_at" TIMESTAMP;
ALTER TABLE "restore_operations" ADD COLUMN "discarded_at" TIMESTAMP;
ALTER TABLE "thumbnail_generation_jobs" ADD COLUMN "discarded_at" TIMESTAMP;
CREATE INDEX "idx_intake_sources_failed" ON "intake_sources" ("group_id", "updated_at") WHERE status = 'failed' AND discarded_at IS NULL;