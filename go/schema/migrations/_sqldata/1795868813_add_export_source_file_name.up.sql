-- Migration generated from schema differences
-- Generated on: 2026-11-28T13:26:53+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "exports" ADD COLUMN "source_file_name" TEXT NOT NULL DEFAULT '';