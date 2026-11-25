-- Migration generated from schema differences
-- Generated on: 2026-11-25T11:01:19+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: saved_views --
CREATE TABLE "saved_views" (
  "name" TEXT NOT NULL,
  "icon" TEXT NOT NULL DEFAULT 'bookmark',
  "entity" TEXT NOT NULL DEFAULT 'commodities',
  "query" TEXT NOT NULL DEFAULT '',
  "query_version" INTEGER NOT NULL DEFAULT 1,
  "layout" TEXT NOT NULL DEFAULT '',
  "shared" BOOLEAN NOT NULL DEFAULT false,
  "sort_order" INTEGER NOT NULL DEFAULT 0,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: saved_view_placements --
CREATE TABLE "saved_view_placements" (
  "group_id" TEXT NOT NULL,
  "saved_view_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "show_in_sidebar" BOOLEAN NOT NULL DEFAULT false,
  "show_on_dashboard" BOOLEAN NOT NULL DEFAULT false,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for saved_view_placements table
ALTER TABLE "saved_view_placements" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for saved_views table
ALTER TABLE "saved_views" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all saved view placements for purges
DROP POLICY IF EXISTS "saved_view_placement_background_worker_access" ON "saved_view_placements";
CREATE POLICY "saved_view_placement_background_worker_access" ON "saved_view_placements" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures saved view placements can only be accessed and modified by their tenant and group with required contexts; the member filter is applied in application logic
DROP POLICY IF EXISTS "saved_view_placement_isolation" ON "saved_view_placements";
CREATE POLICY "saved_view_placement_isolation" ON "saved_view_placements" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all saved views for purges
DROP POLICY IF EXISTS "saved_view_background_worker_access" ON "saved_views";
CREATE POLICY "saved_view_background_worker_access" ON "saved_views" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures saved views can only be accessed and modified by their tenant and group with required contexts; per-user visibility is applied in application logic
DROP POLICY IF EXISTS "saved_view_isolation" ON "saved_views";
CREATE POLICY "saved_view_isolation" ON "saved_views" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS "idx_saved_view_placements_group_user" ON "saved_view_placements" ("tenant_id", "group_id", "user_id");
CREATE INDEX IF NOT EXISTS "idx_saved_view_placements_user" ON "saved_view_placements" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_saved_view_placements_uuid" ON "saved_view_placements" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_saved_view_placements_view_user" ON "saved_view_placements" ("saved_view_id", "user_id");
CREATE INDEX IF NOT EXISTS "idx_saved_views_created_by" ON "saved_views" ("created_by_user_id");
CREATE INDEX IF NOT EXISTS "idx_saved_views_tenant_group_order" ON "saved_views" ("tenant_id", "group_id", "sort_order");
CREATE INDEX IF NOT EXISTS "idx_saved_views_tenant_id" ON "saved_views" ("tenant_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_saved_views_uuid" ON "saved_views" ("uuid");
-- ALTER statements: --
ALTER TABLE "saved_views" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "saved_views" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "saved_views" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "saved_view_placements" ADD CONSTRAINT "fk_saved_view_placement_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "saved_view_placements" ADD CONSTRAINT "fk_saved_view_placement_view" FOREIGN KEY ("saved_view_id") REFERENCES "saved_views"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "saved_view_placements" ADD CONSTRAINT "fk_saved_view_placement_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "saved_view_placements" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");