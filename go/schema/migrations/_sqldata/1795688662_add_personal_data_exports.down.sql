-- Migration rollback
-- Generated on: 2026-11-26T11:24:23+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "personal_data_exports" DROP CONSTRAINT "fk_entity_tenant";
ALTER TABLE "personal_data_exports" DROP CONSTRAINT "fk_entity_user";
DROP INDEX "idx_personal_data_exports_expires_at";
DROP INDEX "idx_personal_data_exports_status";
DROP INDEX "idx_personal_data_exports_user_created";
DROP INDEX "idx_personal_data_exports_uuid";
DROP POLICY "personal_data_export_background_worker_access" ON "personal_data_exports";
DROP POLICY "personal_data_export_isolation" ON "personal_data_exports";
DROP TABLE "personal_data_exports";