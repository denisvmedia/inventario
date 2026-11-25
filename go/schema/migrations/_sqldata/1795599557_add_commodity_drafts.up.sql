-- Migration generated from schema differences
-- Generated on: 2026-11-25T10:39:17+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: commodity_drafts --
CREATE TABLE "commodity_drafts" (
  "client_key" TEXT NOT NULL,
  "form_values" JSONB NOT NULL,
  "revision" INTEGER NOT NULL DEFAULT 1,
  "status" TEXT NOT NULL DEFAULT 'open',
  "promoted_commodity_id" TEXT,
  "promoted_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for commodity_drafts table
ALTER TABLE "commodity_drafts" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to read and purge commodity drafts across all groups
DROP POLICY IF EXISTS "commodity_draft_background_worker_access" ON "commodity_drafts";
CREATE POLICY "commodity_draft_background_worker_access" ON "commodity_drafts" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures commodity drafts can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "commodity_draft_isolation" ON "commodity_drafts";
CREATE POLICY "commodity_draft_isolation" ON "commodity_drafts" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_drafts_group_client_key" ON "commodity_drafts" ("group_id", "client_key");
CREATE INDEX IF NOT EXISTS "idx_commodity_drafts_group_status_updated" ON "commodity_drafts" ("group_id", "status", "updated_at");
CREATE INDEX IF NOT EXISTS "idx_commodity_drafts_tenant_group" ON "commodity_drafts" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_drafts_uuid" ON "commodity_drafts" ("uuid");
-- ALTER statements: --
ALTER TABLE "commodity_drafts" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_drafts" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodity_drafts" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");