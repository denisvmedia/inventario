-- Migration rollback
-- Generated on: 2026-11-24T22:39:04+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_api_tokens_token_hash";
DROP INDEX IF EXISTS "idx_api_tokens_user_id";
DROP INDEX IF EXISTS "idx_api_tokens_uuid";
DROP INDEX IF EXISTS "audit_logs_api_token_id_idx";
-- Drop RLS policy api_token_background_worker_access from table api_tokens
DROP POLICY IF EXISTS "api_token_background_worker_access" ON "api_tokens";
-- Drop RLS policy api_token_isolation from table api_tokens
DROP POLICY IF EXISTS "api_token_isolation" ON "api_tokens";
-- Remove columns from table: audit_logs --
-- ALTER statements: --
ALTER TABLE "audit_logs" DROP COLUMN "api_token_id" CASCADE;
-- WARNING: Dropping column audit_logs.api_token_id with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE "api_tokens" DROP CONSTRAINT IF EXISTS "fk_api_token_group";
-- ALTER statements: --
ALTER TABLE "api_tokens" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "api_tokens" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "api_tokens" CASCADE;