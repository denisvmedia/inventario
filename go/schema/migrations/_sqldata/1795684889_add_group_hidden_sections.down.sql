-- Migration rollback
-- Generated on: 2026-11-26T10:21:29+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "location_groups" DROP COLUMN "hidden_sections";