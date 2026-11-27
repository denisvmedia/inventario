-- Migration generated from schema differences
-- Generated on: 2026-11-27T17:18:55+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "intake_candidates" ADD COLUMN "custom_fields" JSONB NOT NULL DEFAULT '{}';
ALTER TABLE "intake_candidates" ADD COLUMN "accepted_custom_fields" JSONB;