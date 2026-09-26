-- Migration rollback
-- Generated on: 2026-09-26T11:58:15+02:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_weekly_digest_sends_user_week";
DROP INDEX IF EXISTS "idx_weekly_digest_sends_week_start";
-- Drop RLS policy weekly_digest_send_background_worker_access from table weekly_digest_sends
DROP POLICY IF EXISTS "weekly_digest_send_background_worker_access" ON "weekly_digest_sends";
-- Drop RLS policy weekly_digest_send_isolation from table weekly_digest_sends
DROP POLICY IF EXISTS "weekly_digest_send_isolation" ON "weekly_digest_sends";
-- ALTER statements: --
ALTER TABLE "weekly_digest_sends" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "weekly_digest_sends" DROP CONSTRAINT IF EXISTS "fk_entity_user";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "weekly_digest_sends" CASCADE;