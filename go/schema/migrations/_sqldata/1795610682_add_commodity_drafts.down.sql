-- Migration rollback
-- Generated on: 2026-11-25T13:44:42+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_commodity_drafts_group_client_key";
DROP INDEX IF EXISTS "idx_commodity_drafts_group_status_updated";
DROP INDEX IF EXISTS "idx_commodity_drafts_tenant_group";
DROP INDEX IF EXISTS "idx_commodity_drafts_uuid";
-- Drop RLS policy commodity_draft_background_worker_access from table commodity_drafts
DROP POLICY IF EXISTS "commodity_draft_background_worker_access" ON "commodity_drafts";
-- Drop RLS policy commodity_draft_isolation from table commodity_drafts
DROP POLICY IF EXISTS "commodity_draft_isolation" ON "commodity_drafts";
-- ALTER statements: --
ALTER TABLE "commodity_drafts" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_drafts" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodity_drafts" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_drafts" CASCADE;