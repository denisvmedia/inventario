-- Migration generated from schema differences
-- Generated on: 2026-11-24T23:37:04+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: file_links --
CREATE TABLE "file_links" (
  "file_id" TEXT NOT NULL,
  "entity_type" TEXT NOT NULL,
  "entity_id" TEXT NOT NULL,
  "meta" TEXT NOT NULL DEFAULT '',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE "files" ADD COLUMN "sha256" TEXT NOT NULL DEFAULT '';
-- Enable RLS for file_links table
ALTER TABLE "file_links" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all file links for processing
DROP POLICY IF EXISTS "file_link_background_worker_access" ON "file_links";
CREATE POLICY "file_link_background_worker_access" ON "file_links" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures file links can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "file_link_isolation" ON "file_links";
CREATE POLICY "file_link_isolation" ON "file_links" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS "idx_file_links_entity" ON "file_links" ("entity_type", "entity_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_file_links_file_entity" ON "file_links" ("file_id", "entity_type", "entity_id");
CREATE INDEX IF NOT EXISTS "idx_file_links_tenant_group" ON "file_links" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_file_links_uuid" ON "file_links" ("uuid");
CREATE INDEX IF NOT EXISTS "idx_files_tenant_group_sha256" ON "files" ("tenant_id", "group_id", "sha256");
-- ALTER statements: --
ALTER TABLE "file_links" ADD CONSTRAINT "fk_file_link_file" FOREIGN KEY ("file_id") REFERENCES "files"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "file_links" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "file_links" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "file_links" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");