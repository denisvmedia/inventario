-- Migration generated from schema differences
-- Generated on: 2026-11-24T22:39:04+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: api_tokens --
CREATE TABLE "api_tokens" (
  "name" VARCHAR(100) NOT NULL,
  "token_hash" VARCHAR(64) NOT NULL,
  "token_prefix" VARCHAR(16) NOT NULL,
  "scope" VARCHAR(16) NOT NULL,
  "group_id" TEXT,
  "expires_at" TIMESTAMP,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "last_used_at" TIMESTAMP,
  "revoked_at" TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Add/modify columns for table: audit_logs --
-- ALTER statements: --
ALTER TABLE "audit_logs" ADD COLUMN "api_token_id" TEXT;
-- Enable RLS for api_tokens table
ALTER TABLE "api_tokens" ENABLE ROW LEVEL SECURITY;
-- Allows token authentication to look tokens up by hash before a user context exists
DROP POLICY IF EXISTS "api_token_background_worker_access" ON "api_tokens";
CREATE POLICY "api_token_background_worker_access" ON "api_tokens" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures API tokens can only be accessed and modified by the owning user within their tenant
DROP POLICY IF EXISTS "api_token_isolation" ON "api_tokens";
CREATE POLICY "api_token_isolation" ON "api_tokens" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
CREATE UNIQUE INDEX IF NOT EXISTS "idx_api_tokens_token_hash" ON "api_tokens" ("token_hash");
CREATE INDEX IF NOT EXISTS "idx_api_tokens_user_id" ON "api_tokens" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_api_tokens_uuid" ON "api_tokens" ("uuid");
CREATE INDEX IF NOT EXISTS "audit_logs_api_token_id_idx" ON "audit_logs" ("api_token_id");
-- ALTER statements: --
ALTER TABLE "api_tokens" ADD CONSTRAINT "fk_api_token_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "api_tokens" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "api_tokens" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");