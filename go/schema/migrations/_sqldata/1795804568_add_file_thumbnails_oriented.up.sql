-- Migration generated from schema differences
-- Generated on: 2026-11-27T19:36:08+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "files" ADD COLUMN "thumbnails_oriented" BOOLEAN NOT NULL DEFAULT false;