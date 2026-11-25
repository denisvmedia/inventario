-- Migration generated from schema differences
-- Generated on: 2026-11-25T09:08:37+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: sellers --
CREATE TABLE "sellers" (
  "name" TEXT NOT NULL,
  "name_key" TEXT NOT NULL,
  "website" TEXT NOT NULL DEFAULT '',
  "contact" TEXT NOT NULL DEFAULT '',
  "note" TEXT NOT NULL DEFAULT '',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "seller_id" TEXT;
-- Add foreign key constraints for table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" ADD CONSTRAINT "fk_commodity_seller" FOREIGN KEY ("seller_id") REFERENCES "sellers"("id") ON DELETE SET NULL;
-- Enable RLS for sellers table
ALTER TABLE "sellers" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all sellers for processing
DROP POLICY IF EXISTS "seller_background_worker_access" ON "sellers";
CREATE POLICY "seller_background_worker_access" ON "sellers" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures sellers can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "seller_isolation" ON "sellers";
CREATE POLICY "seller_isolation" ON "sellers" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS "idx_commodities_seller" ON "commodities" ("seller_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_sellers_group_name_key" ON "sellers" ("group_id", "name_key");
CREATE INDEX IF NOT EXISTS "idx_sellers_tenant_group" ON "sellers" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_sellers_uuid" ON "sellers" ("uuid");
-- ALTER statements: --
ALTER TABLE "sellers" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "sellers" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "sellers" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");