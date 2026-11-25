-- Migration generated from schema differences
-- Generated on: 2026-11-25T17:48:32+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: intake_sessions --
CREATE TABLE "intake_sessions" (
  "client_key" TEXT NOT NULL,
  "origin" TEXT NOT NULL DEFAULT 'upload',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: intake_sources --
CREATE TABLE "intake_sources" (
  "session_id" TEXT NOT NULL,
  "sort_order" INTEGER NOT NULL DEFAULT 0,
  "one_item" BOOLEAN NOT NULL DEFAULT false,
  "status" TEXT NOT NULL DEFAULT 'pending',
  "attempts" INTEGER NOT NULL DEFAULT 0,
  "next_attempt_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "claimed_at" TIMESTAMP,
  "wait_reason" TEXT NOT NULL DEFAULT '',
  "error_code" TEXT NOT NULL DEFAULT '',
  "warnings" JSONB,
  "scanned_at" TIMESTAMP,
  "external_system" TEXT NOT NULL DEFAULT '',
  "external_instance" TEXT NOT NULL DEFAULT '',
  "external_document_id" TEXT NOT NULL DEFAULT '',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: intake_candidates --
CREATE TABLE "intake_candidates" (
  "session_id" TEXT NOT NULL,
  "source_id" TEXT NOT NULL,
  "sort_order" INTEGER NOT NULL DEFAULT 0,
  "fields" JSONB NOT NULL,
  "selected" BOOLEAN NOT NULL DEFAULT true,
  "accepted_fields" JSONB,
  "quantity" INTEGER NOT NULL DEFAULT 1,
  "outcome" TEXT NOT NULL DEFAULT 'pending',
  "draft_id" TEXT,
  "error_code" TEXT NOT NULL DEFAULT '',
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for intake_candidates table
ALTER TABLE "intake_candidates" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for intake_sessions table
ALTER TABLE "intake_sessions" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for intake_sources table
ALTER TABLE "intake_sources" ENABLE ROW LEVEL SECURITY;
-- Allows the intake worker to store detected products across all groups
DROP POLICY IF EXISTS "intake_candidate_background_worker_access" ON "intake_candidates";
CREATE POLICY "intake_candidate_background_worker_access" ON "intake_candidates" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures intake candidates can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "intake_candidate_isolation" ON "intake_candidates";
CREATE POLICY "intake_candidate_isolation" ON "intake_candidates" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to read and purge intake sessions across all groups
DROP POLICY IF EXISTS "intake_session_background_worker_access" ON "intake_sessions";
CREATE POLICY "intake_session_background_worker_access" ON "intake_sessions" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures intake sessions can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "intake_session_isolation" ON "intake_sessions";
CREATE POLICY "intake_session_isolation" ON "intake_sessions" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows the intake worker to claim and update intake sources across all groups
DROP POLICY IF EXISTS "intake_source_background_worker_access" ON "intake_sources";
CREATE POLICY "intake_source_background_worker_access" ON "intake_sources" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures intake sources can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "intake_source_isolation" ON "intake_sources";
CREATE POLICY "intake_source_isolation" ON "intake_sources" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS "idx_intake_candidates_session" ON "intake_candidates" ("session_id");
CREATE INDEX IF NOT EXISTS "idx_intake_candidates_source_order" ON "intake_candidates" ("source_id", "sort_order");
CREATE INDEX IF NOT EXISTS "idx_intake_candidates_tenant_group" ON "intake_candidates" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_intake_candidates_uuid" ON "intake_candidates" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_intake_sessions_group_client_key" ON "intake_sessions" ("group_id", "client_key");
CREATE INDEX IF NOT EXISTS "idx_intake_sessions_group_created" ON "intake_sessions" ("group_id", "created_at");
CREATE INDEX IF NOT EXISTS "idx_intake_sessions_tenant_group" ON "intake_sessions" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_intake_sessions_uuid" ON "intake_sessions" ("uuid");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_intake_sources_external" ON "intake_sources" ("group_id", "external_system", "external_instance", "external_document_id") WHERE external_document_id <> '';
CREATE INDEX IF NOT EXISTS "idx_intake_sources_queue" ON "intake_sources" ("status", "next_attempt_at") WHERE status IN ('pending', 'scanning');
CREATE INDEX IF NOT EXISTS "idx_intake_sources_session_order" ON "intake_sources" ("session_id", "sort_order");
CREATE INDEX IF NOT EXISTS "idx_intake_sources_tenant_group" ON "intake_sources" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_intake_sources_uuid" ON "intake_sources" ("uuid");
-- ALTER statements: --
ALTER TABLE "intake_sessions" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "intake_sessions" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "intake_sessions" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "intake_sources" ADD CONSTRAINT "fk_intake_source_session" FOREIGN KEY ("session_id") REFERENCES "intake_sessions"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "intake_sources" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "intake_sources" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "intake_sources" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "intake_candidates" ADD CONSTRAINT "fk_intake_candidate_session" FOREIGN KEY ("session_id") REFERENCES "intake_sessions"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "intake_candidates" ADD CONSTRAINT "fk_intake_candidate_source" FOREIGN KEY ("source_id") REFERENCES "intake_sources"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "intake_candidates" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "intake_candidates" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "intake_candidates" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");