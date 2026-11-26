-- Migration rollback
-- Generated on: 2026-11-26T16:35:09+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "custom_field_definitions" DROP COLUMN "set_entry";
ALTER TABLE "item_types" DROP COLUMN "set_entry";