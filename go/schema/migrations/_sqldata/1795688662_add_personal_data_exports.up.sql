-- Migration generated from schema differences
-- Generated on: 2026-11-26T11:24:23+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

CREATE TABLE "personal_data_exports" (
  "status" VARCHAR(16) NOT NULL DEFAULT 'pending',
  "blob_key" TEXT NOT NULL DEFAULT '',
  "size_bytes" BIGINT NOT NULL DEFAULT 0,
  "group_count" INTEGER NOT NULL DEFAULT 0,
  "item_count" INTEGER NOT NULL DEFAULT 0,
  "file_count" INTEGER NOT NULL DEFAULT 0,
  "error_message" TEXT NOT NULL DEFAULT '',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "completed_at" TIMESTAMP,
  "expires_at" TIMESTAMP NOT NULL,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for multi-tenant personal data export isolation
ALTER TABLE "personal_data_exports" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "personal_data_export_background_worker_access" ON "personal_data_exports" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
COMMENT ON POLICY "personal_data_export_background_worker_access" ON "personal_data_exports" IS 'Allows the export worker to build and expire the personal data exports of every user';
CREATE POLICY "personal_data_export_isolation" ON "personal_data_exports" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
COMMENT ON POLICY "personal_data_export_isolation" ON "personal_data_exports" IS 'Ensures personal data exports can only be accessed by the owning user within their tenant';
CREATE INDEX "idx_personal_data_exports_expires_at" ON "personal_data_exports" ("expires_at");
CREATE INDEX "idx_personal_data_exports_status" ON "personal_data_exports" ("status");
CREATE INDEX "idx_personal_data_exports_user_created" ON "personal_data_exports" ("user_id", "created_at");
CREATE UNIQUE INDEX "idx_personal_data_exports_uuid" ON "personal_data_exports" ("uuid");
ALTER TABLE "personal_data_exports" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
ALTER TABLE "personal_data_exports" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");