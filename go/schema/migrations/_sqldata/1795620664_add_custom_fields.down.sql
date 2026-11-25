-- Migration rollback
-- Generated on: 2026-11-25T16:31:04+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_commodity_custom_field_values_commodity_field";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_field_bool";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_field_date";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_field_item";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_field_money";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_field_number";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_field_select";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_field_text";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_item";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_long_text_trgm";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_tenant_group";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_text_trgm";
DROP INDEX IF EXISTS "idx_commodity_custom_field_values_uuid";
DROP INDEX IF EXISTS "idx_custom_field_definitions_group_name_key";
DROP INDEX IF EXISTS "idx_custom_field_definitions_tenant_group";
DROP INDEX IF EXISTS "idx_custom_field_definitions_uuid";
DROP INDEX IF EXISTS "idx_item_type_custom_fields_field";
DROP INDEX IF EXISTS "idx_item_type_custom_fields_tenant_group";
DROP INDEX IF EXISTS "idx_item_type_custom_fields_type_field";
DROP INDEX IF EXISTS "idx_item_type_custom_fields_uuid";
-- Drop RLS policy commodity_custom_field_value_background_worker_access from table commodity_custom_field_values
DROP POLICY IF EXISTS "commodity_custom_field_value_background_worker_access" ON "commodity_custom_field_values";
-- Drop RLS policy commodity_custom_field_value_isolation from table commodity_custom_field_values
DROP POLICY IF EXISTS "commodity_custom_field_value_isolation" ON "commodity_custom_field_values";
-- Drop RLS policy custom_field_definition_background_worker_access from table custom_field_definitions
DROP POLICY IF EXISTS "custom_field_definition_background_worker_access" ON "custom_field_definitions";
-- Drop RLS policy custom_field_definition_isolation from table custom_field_definitions
DROP POLICY IF EXISTS "custom_field_definition_isolation" ON "custom_field_definitions";
-- Drop RLS policy item_type_custom_field_background_worker_access from table item_type_custom_fields
DROP POLICY IF EXISTS "item_type_custom_field_background_worker_access" ON "item_type_custom_fields";
-- Drop RLS policy item_type_custom_field_isolation from table item_type_custom_fields
DROP POLICY IF EXISTS "item_type_custom_field_isolation" ON "item_type_custom_fields";
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT IF EXISTS "fk_custom_field_value_commodity";
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT IF EXISTS "fk_custom_field_value_field";
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT IF EXISTS "fk_item_type_custom_field_type";
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT IF EXISTS "fk_item_type_custom_field_field";
-- ALTER statements: --
ALTER TABLE "custom_field_definitions" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "custom_field_definitions" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "custom_field_definitions" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT IF EXISTS "fk_custom_field_value_item";
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_custom_field_values" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "item_type_custom_fields" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "custom_field_definitions" CASCADE;