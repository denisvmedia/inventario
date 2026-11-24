-- Migration generated from schema differences
-- Generated on: 2026-11-24T22:34:21+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "asset_number" INTEGER;
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodities_group_asset_number" ON "commodities" ("group_id", "asset_number");