-- Migration rollback
-- Generated on: 2026-11-25T17:48:32+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_intake_candidates_session";
DROP INDEX IF EXISTS "idx_intake_candidates_source_order";
DROP INDEX IF EXISTS "idx_intake_candidates_tenant_group";
DROP INDEX IF EXISTS "idx_intake_candidates_uuid";
DROP INDEX IF EXISTS "idx_intake_sessions_group_client_key";
DROP INDEX IF EXISTS "idx_intake_sessions_group_created";
DROP INDEX IF EXISTS "idx_intake_sessions_tenant_group";
DROP INDEX IF EXISTS "idx_intake_sessions_uuid";
DROP INDEX IF EXISTS "idx_intake_sources_external";
DROP INDEX IF EXISTS "idx_intake_sources_queue";
DROP INDEX IF EXISTS "idx_intake_sources_session_order";
DROP INDEX IF EXISTS "idx_intake_sources_tenant_group";
DROP INDEX IF EXISTS "idx_intake_sources_uuid";
-- Drop RLS policy intake_candidate_background_worker_access from table intake_candidates
DROP POLICY IF EXISTS "intake_candidate_background_worker_access" ON "intake_candidates";
-- Drop RLS policy intake_candidate_isolation from table intake_candidates
DROP POLICY IF EXISTS "intake_candidate_isolation" ON "intake_candidates";
-- Drop RLS policy intake_session_background_worker_access from table intake_sessions
DROP POLICY IF EXISTS "intake_session_background_worker_access" ON "intake_sessions";
-- Drop RLS policy intake_session_isolation from table intake_sessions
DROP POLICY IF EXISTS "intake_session_isolation" ON "intake_sessions";
-- Drop RLS policy intake_source_background_worker_access from table intake_sources
DROP POLICY IF EXISTS "intake_source_background_worker_access" ON "intake_sources";
-- Drop RLS policy intake_source_isolation from table intake_sources
DROP POLICY IF EXISTS "intake_source_isolation" ON "intake_sources";
-- ALTER statements: --
ALTER TABLE "intake_sources" DROP CONSTRAINT IF EXISTS "fk_intake_source_session";
-- ALTER statements: --
ALTER TABLE "intake_candidates" DROP CONSTRAINT IF EXISTS "fk_intake_candidate_session";
-- ALTER statements: --
ALTER TABLE "intake_candidates" DROP CONSTRAINT IF EXISTS "fk_intake_candidate_source";
-- ALTER statements: --
ALTER TABLE "intake_candidates" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "intake_candidates" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "intake_candidates" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "intake_sessions" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "intake_sessions" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "intake_sessions" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "intake_sources" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "intake_sources" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "intake_sources" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "intake_candidates" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "intake_sources" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "intake_sessions" CASCADE;