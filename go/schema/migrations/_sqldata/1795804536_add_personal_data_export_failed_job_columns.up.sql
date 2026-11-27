-- Migration generated from schema differences
-- Generated on: 2026-11-27T19:35:36+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "personal_data_exports" ADD COLUMN "attempt_count" INTEGER NOT NULL DEFAULT 0;
ALTER TABLE "personal_data_exports" ADD COLUMN "discarded_at" TIMESTAMP;