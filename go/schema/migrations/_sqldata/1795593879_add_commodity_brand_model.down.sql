-- Migration rollback
-- Generated on: 2026-11-25T09:04:39+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "commodities_brand_trgm_idx";
DROP INDEX IF EXISTS "commodities_model_trgm_idx";
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "brand" CASCADE;
-- WARNING: Dropping column commodities.brand with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "model" CASCADE;
-- WARNING: Dropping column commodities.model with CASCADE - This will delete data and dependent objects! --;