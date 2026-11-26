-- Migration generated from schema differences
-- Generated on: 2026-11-26T18:19:13+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "custom_field_definitions" ADD COLUMN "set_entry" TEXT NOT NULL DEFAULT '';
ALTER TABLE "item_types" ADD COLUMN "set_entry" TEXT NOT NULL DEFAULT '';