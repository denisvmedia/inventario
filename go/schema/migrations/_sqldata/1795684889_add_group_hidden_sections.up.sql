-- Migration generated from schema differences
-- Generated on: 2026-11-26T10:21:29+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "location_groups" ADD COLUMN "hidden_sections" JSONB NOT NULL DEFAULT '[]';