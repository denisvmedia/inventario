-- Migration rollback
-- Generated on: 2026-11-25T09:08:37+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_commodities_seller";
DROP INDEX IF EXISTS "idx_sellers_group_name_key";
DROP INDEX IF EXISTS "idx_sellers_tenant_group";
DROP INDEX IF EXISTS "idx_sellers_uuid";
-- Drop RLS policy seller_background_worker_access from table sellers
DROP POLICY IF EXISTS "seller_background_worker_access" ON "sellers";
-- Drop RLS policy seller_isolation from table sellers
DROP POLICY IF EXISTS "seller_isolation" ON "sellers";
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "seller_id" CASCADE;
-- WARNING: Dropping column commodities.seller_id with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE "sellers" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "sellers" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "sellers" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "commodities" DROP CONSTRAINT IF EXISTS "fk_commodity_seller";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "sellers" CASCADE;