-- Migration rollback
-- Generated on: 2026-11-25T12:01:58+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_commodities_group_type";
DROP INDEX IF EXISTS "idx_item_types_group_name_key";
DROP INDEX IF EXISTS "idx_item_types_group_slug";
DROP INDEX IF EXISTS "idx_item_types_tenant_group";
DROP INDEX IF EXISTS "idx_item_types_uuid";
-- Drop RLS policy item_type_background_worker_access from table item_types
DROP POLICY IF EXISTS "item_type_background_worker_access" ON "item_types";
-- Drop RLS policy item_type_isolation from table item_types
DROP POLICY IF EXISTS "item_type_isolation" ON "item_types";
-- ALTER statements: --
ALTER TABLE "item_types" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "item_types" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "item_types" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "item_types" CASCADE;