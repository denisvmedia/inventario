-- Migration generated from schema differences
-- Generated on: 2026-11-25T12:01:58+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: item_types --
CREATE TABLE "item_types" (
  "slug" TEXT NOT NULL,
  "name" TEXT NOT NULL DEFAULT '',
  "name_key" TEXT NOT NULL,
  "icon" TEXT NOT NULL DEFAULT '',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for item_types table
ALTER TABLE "item_types" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all item types for processing
DROP POLICY IF EXISTS "item_type_background_worker_access" ON "item_types";
CREATE POLICY "item_type_background_worker_access" ON "item_types" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures item types can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "item_type_isolation" ON "item_types";
CREATE POLICY "item_type_isolation" ON "item_types" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS "idx_commodities_group_type" ON "commodities" ("group_id", "type");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_item_types_group_name_key" ON "item_types" ("group_id", "name_key");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_item_types_group_slug" ON "item_types" ("group_id", "slug");
CREATE INDEX IF NOT EXISTS "idx_item_types_tenant_group" ON "item_types" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_item_types_uuid" ON "item_types" ("uuid");
-- ALTER statements: --
ALTER TABLE "item_types" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "item_types" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "item_types" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");