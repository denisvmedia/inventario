-- Migration rollback
-- Generated on: 2026-11-24T22:34:21+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_commodities_group_asset_number";
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "asset_number" CASCADE;
-- WARNING: Dropping column commodities.asset_number with CASCADE - This will delete data and dependent objects! --;