-- Migration rollback
-- Generated on: 2026-11-26T01:16:17+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "intake_documents" DROP CONSTRAINT "fk_entity_tenant";
ALTER TABLE "intake_documents" DROP CONSTRAINT "fk_entity_group";
ALTER TABLE "intake_documents" DROP CONSTRAINT "fk_entity_created_by";
DROP INDEX "idx_intake_documents_external";
DROP INDEX "idx_intake_documents_group_commodity";
DROP INDEX "idx_intake_documents_group_draft";
DROP INDEX "idx_intake_documents_tenant_group";
DROP INDEX "idx_intake_documents_uuid";
DROP POLICY "intake_document_background_worker_access" ON "intake_documents";
DROP POLICY "intake_document_isolation" ON "intake_documents";
DROP TABLE "intake_documents";