-- Migration generated from schema differences
-- Generated on: 2026-11-26T01:16:17+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

CREATE TABLE "intake_documents" (
  "external_system" TEXT NOT NULL,
  "external_instance" TEXT NOT NULL DEFAULT '',
  "external_document_id" TEXT NOT NULL,
  "source_name" TEXT NOT NULL DEFAULT '',
  "source_url" TEXT NOT NULL DEFAULT '',
  "payload_hash" TEXT NOT NULL,
  "file_id" TEXT,
  "session_id" TEXT,
  "draft_id" TEXT,
  "commodity_id" TEXT,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for multi-tenant intake document isolation
ALTER TABLE "intake_documents" ENABLE ROW LEVEL SECURITY;
CREATE POLICY "intake_document_background_worker_access" ON "intake_documents" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
COMMENT ON POLICY "intake_document_background_worker_access" ON "intake_documents" IS 'Allows background workers to read and purge intake documents across all groups';
CREATE POLICY "intake_document_isolation" ON "intake_documents" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
COMMENT ON POLICY "intake_document_isolation" ON "intake_documents" IS 'Ensures intake documents can only be accessed and modified by their tenant and group with required contexts';
CREATE UNIQUE INDEX "idx_intake_documents_external" ON "intake_documents" ("group_id", "external_system", "external_instance", "external_document_id");
CREATE INDEX "idx_intake_documents_group_commodity" ON "intake_documents" ("group_id", "commodity_id") WHERE commodity_id IS NOT NULL;
CREATE INDEX "idx_intake_documents_group_draft" ON "intake_documents" ("group_id", "draft_id") WHERE draft_id IS NOT NULL;
CREATE INDEX "idx_intake_documents_tenant_group" ON "intake_documents" ("tenant_id", "group_id");
CREATE UNIQUE INDEX "idx_intake_documents_uuid" ON "intake_documents" ("uuid");
ALTER TABLE "intake_documents" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
ALTER TABLE "intake_documents" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
ALTER TABLE "intake_documents" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");