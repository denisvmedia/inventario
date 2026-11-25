-- Migration generated from schema differences
-- Generated on: 2026-11-25T16:31:04+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: custom_field_definitions --
CREATE TABLE "custom_field_definitions" (
  "name" TEXT NOT NULL,
  "name_key" TEXT NOT NULL,
  "type" TEXT NOT NULL,
  "options" JSONB,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: commodity_custom_field_values --
CREATE TABLE "commodity_custom_field_values" (
  "group_id" TEXT NOT NULL,
  "commodity_id" TEXT NOT NULL,
  "field_id" TEXT NOT NULL,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
  "value_text" TEXT,
  "value_long_text" TEXT,
  "value_number" NUMERIC,
  "value_date" DATE,
  "value_bool" BOOLEAN,
  "value_select" TEXT,
  "value_money" DECIMAL(15,2),
  "value_currency" TEXT,
  "value_item_id" TEXT
);
-- POSTGRES TABLE: item_type_custom_fields --
CREATE TABLE "item_type_custom_fields" (
  "item_type_id" TEXT NOT NULL,
  "field_id" TEXT NOT NULL,
  "default_value" JSONB,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for commodity_custom_field_values table
ALTER TABLE "commodity_custom_field_values" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for custom_field_definitions table
ALTER TABLE "custom_field_definitions" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for item_type_custom_fields table
ALTER TABLE "item_type_custom_fields" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all custom field values for processing
DROP POLICY IF EXISTS "commodity_custom_field_value_background_worker_access" ON "commodity_custom_field_values";
CREATE POLICY "commodity_custom_field_value_background_worker_access" ON "commodity_custom_field_values" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures custom field values can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "commodity_custom_field_value_isolation" ON "commodity_custom_field_values";
CREATE POLICY "commodity_custom_field_value_isolation" ON "commodity_custom_field_values" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all custom field definitions for processing
DROP POLICY IF EXISTS "custom_field_definition_background_worker_access" ON "custom_field_definitions";
CREATE POLICY "custom_field_definition_background_worker_access" ON "custom_field_definitions" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures custom field definitions can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "custom_field_definition_isolation" ON "custom_field_definitions";
CREATE POLICY "custom_field_definition_isolation" ON "custom_field_definitions" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all item type templates for processing
DROP POLICY IF EXISTS "item_type_custom_field_background_worker_access" ON "item_type_custom_fields";
CREATE POLICY "item_type_custom_field_background_worker_access" ON "item_type_custom_fields" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures item type templates can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "item_type_custom_field_isolation" ON "item_type_custom_fields";
CREATE POLICY "item_type_custom_field_isolation" ON "item_type_custom_fields" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_commodity_field" ON "commodity_custom_field_values" ("commodity_id", "field_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_field_bool" ON "commodity_custom_field_values" ("field_id", "value_bool");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_field_date" ON "commodity_custom_field_values" ("field_id", "value_date");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_field_item" ON "commodity_custom_field_values" ("field_id", "value_item_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_field_money" ON "commodity_custom_field_values" ("field_id", "value_money");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_field_number" ON "commodity_custom_field_values" ("field_id", "value_number");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_field_select" ON "commodity_custom_field_values" ("field_id", "value_select");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_field_text" ON "commodity_custom_field_values" ("field_id", "value_text");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_item" ON "commodity_custom_field_values" ("value_item_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_long_text_trgm" ON "commodity_custom_field_values" USING GIN ("value_long_text" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_tenant_group" ON "commodity_custom_field_values" ("tenant_id", "group_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_text_trgm" ON "commodity_custom_field_values" USING GIN ("value_text" gin_trgm_ops);
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_custom_field_values_uuid" ON "commodity_custom_field_values" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_custom_field_definitions_group_name_key" ON "custom_field_definitions" ("group_id", "name_key");
CREATE INDEX IF NOT EXISTS "idx_custom_field_definitions_tenant_group" ON "custom_field_definitions" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_custom_field_definitions_uuid" ON "custom_field_definitions" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_item_type_custom_fields_field" ON "item_type_custom_fields" ("field_id");
CREATE INDEX IF NOT EXISTS "idx_item_type_custom_fields_tenant_group" ON "item_type_custom_fields" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_item_type_custom_fields_type_field" ON "item_type_custom_fields" ("item_type_id", "field_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_item_type_custom_fields_uuid" ON "item_type_custom_fields" ("uuid");
-- ALTER statements: --
ALTER TABLE "custom_field_definitions" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "custom_field_definitions" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "custom_field_definitions" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" ADD CONSTRAINT "fk_custom_field_value_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" ADD CONSTRAINT "fk_custom_field_value_field" FOREIGN KEY ("field_id") REFERENCES "custom_field_definitions"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_custom_field_values" ADD CONSTRAINT "fk_custom_field_value_item" FOREIGN KEY ("value_item_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" ADD CONSTRAINT "fk_item_type_custom_field_type" FOREIGN KEY ("item_type_id") REFERENCES "item_types"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" ADD CONSTRAINT "fk_item_type_custom_field_field" FOREIGN KEY ("field_id") REFERENCES "custom_field_definitions"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "item_type_custom_fields" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");