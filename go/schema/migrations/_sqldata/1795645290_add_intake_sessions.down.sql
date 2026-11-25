-- Migration rollback
-- Generated on: 2026-11-25T23:21:30+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "intake_sources" DROP CONSTRAINT "fk_intake_source_session";
ALTER TABLE "intake_candidates" DROP CONSTRAINT "fk_intake_candidate_session";
ALTER TABLE "intake_candidates" DROP CONSTRAINT "fk_intake_candidate_source";
ALTER TABLE "intake_candidates" DROP CONSTRAINT "fk_entity_tenant";
ALTER TABLE "intake_candidates" DROP CONSTRAINT "fk_entity_group";
ALTER TABLE "intake_candidates" DROP CONSTRAINT "fk_entity_created_by";
ALTER TABLE "intake_sessions" DROP CONSTRAINT "fk_entity_tenant";
ALTER TABLE "intake_sessions" DROP CONSTRAINT "fk_entity_group";
ALTER TABLE "intake_sessions" DROP CONSTRAINT "fk_entity_created_by";
ALTER TABLE "intake_sources" DROP CONSTRAINT "fk_entity_tenant";
ALTER TABLE "intake_sources" DROP CONSTRAINT "fk_entity_group";
ALTER TABLE "intake_sources" DROP CONSTRAINT "fk_entity_created_by";
DROP INDEX "idx_intake_candidates_session";
DROP INDEX "idx_intake_candidates_source_order";
DROP INDEX "idx_intake_candidates_tenant_group";
DROP INDEX "idx_intake_candidates_uuid";
DROP INDEX "idx_intake_sessions_group_client_key";
DROP INDEX "idx_intake_sessions_group_created";
DROP INDEX "idx_intake_sessions_tenant_group";
DROP INDEX "idx_intake_sessions_uuid";
DROP INDEX "idx_intake_sources_external";
DROP INDEX "idx_intake_sources_queue";
DROP INDEX "idx_intake_sources_session_order";
DROP INDEX "idx_intake_sources_tenant_group";
DROP INDEX "idx_intake_sources_uuid";
DROP POLICY "intake_candidate_background_worker_access" ON "intake_candidates";
DROP POLICY "intake_candidate_isolation" ON "intake_candidates";
DROP POLICY "intake_source_background_worker_access" ON "intake_sources";
DROP POLICY "intake_source_isolation" ON "intake_sources";
DROP POLICY "intake_session_background_worker_access" ON "intake_sessions";
DROP POLICY "intake_session_isolation" ON "intake_sessions";
DROP TABLE "intake_candidates";
DROP TABLE "intake_sources";
DROP TABLE "intake_sessions";