-- Migration rollback
-- Generated on: 2026-11-25T23:22:18+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT "fk_entity_group";
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT "fk_custom_field_value_commodity";
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT "fk_custom_field_value_field";
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT "fk_item_type_custom_field_type";
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT "fk_item_type_custom_field_field";
ALTER TABLE "custom_field_definitions" DROP CONSTRAINT "fk_entity_tenant";
ALTER TABLE "custom_field_definitions" DROP CONSTRAINT "fk_entity_group";
ALTER TABLE "custom_field_definitions" DROP CONSTRAINT "fk_entity_created_by";
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT "fk_entity_tenant";
ALTER TABLE "commodity_custom_field_values" DROP CONSTRAINT "fk_custom_field_value_item";
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT "fk_entity_tenant";
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT "fk_entity_group";
ALTER TABLE "item_type_custom_fields" DROP CONSTRAINT "fk_entity_created_by";
DROP INDEX "idx_commodity_custom_field_values_commodity_field";
DROP INDEX "idx_commodity_custom_field_values_field_bool";
DROP INDEX "idx_commodity_custom_field_values_field_date";
DROP INDEX "idx_commodity_custom_field_values_field_item";
DROP INDEX "idx_commodity_custom_field_values_field_money";
DROP INDEX "idx_commodity_custom_field_values_field_number";
DROP INDEX "idx_commodity_custom_field_values_field_select";
DROP INDEX "idx_commodity_custom_field_values_field_text";
DROP INDEX "idx_commodity_custom_field_values_item";
DROP INDEX "idx_commodity_custom_field_values_long_text_trgm";
DROP INDEX "idx_commodity_custom_field_values_tenant_group";
DROP INDEX "idx_commodity_custom_field_values_text_trgm";
DROP INDEX "idx_commodity_custom_field_values_uuid";
DROP INDEX "idx_custom_field_definitions_group_name_key";
DROP INDEX "idx_custom_field_definitions_tenant_group";
DROP INDEX "idx_custom_field_definitions_uuid";
DROP INDEX "idx_item_type_custom_fields_field";
DROP INDEX "idx_item_type_custom_fields_tenant_group";
DROP INDEX "idx_item_type_custom_fields_type_field";
DROP INDEX "idx_item_type_custom_fields_uuid";
DROP POLICY "commodity_custom_field_value_background_worker_access" ON "commodity_custom_field_values";
DROP POLICY "commodity_custom_field_value_isolation" ON "commodity_custom_field_values";
DROP POLICY "item_type_custom_field_background_worker_access" ON "item_type_custom_fields";
DROP POLICY "item_type_custom_field_isolation" ON "item_type_custom_fields";
DROP POLICY "custom_field_definition_background_worker_access" ON "custom_field_definitions";
DROP POLICY "custom_field_definition_isolation" ON "custom_field_definitions";
DROP TABLE "commodity_custom_field_values";
DROP TABLE "item_type_custom_fields";
DROP TABLE "custom_field_definitions";