-- Migration rollback
-- Generated on: 2026-11-25T06:43:21+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_commodity_reminder_sends_group_id";
DROP INDEX IF EXISTS "idx_commodity_reminder_sends_key";
DROP INDEX IF EXISTS "idx_commodity_reminder_sends_tenant_id";
DROP INDEX IF EXISTS "idx_commodity_reminders_commodity";
DROP INDEX IF EXISTS "idx_commodity_reminders_tenant_group";
DROP INDEX IF EXISTS "idx_commodity_reminders_uuid";
-- Drop RLS policy commodity_reminder_send_background_worker_access from table commodity_reminder_sends
DROP POLICY IF EXISTS "commodity_reminder_send_background_worker_access" ON "commodity_reminder_sends";
-- Drop RLS policy commodity_reminder_send_isolation from table commodity_reminder_sends
DROP POLICY IF EXISTS "commodity_reminder_send_isolation" ON "commodity_reminder_sends";
-- Drop RLS policy commodity_reminder_background_worker_access from table commodity_reminders
DROP POLICY IF EXISTS "commodity_reminder_background_worker_access" ON "commodity_reminders";
-- Drop RLS policy commodity_reminder_isolation from table commodity_reminders
DROP POLICY IF EXISTS "commodity_reminder_isolation" ON "commodity_reminders";
-- ALTER statements: --
ALTER TABLE "commodity_reminders" DROP CONSTRAINT IF EXISTS "fk_commodity_reminder_commodity";
-- ALTER statements: --
ALTER TABLE "commodity_reminder_sends" DROP CONSTRAINT IF EXISTS "fk_commodity_reminder_send_reminder";
-- ALTER statements: --
ALTER TABLE "commodity_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodity_reminders" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "commodity_reminder_sends" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "commodity_reminder_sends" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "commodity_reminder_sends" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_reminder_sends" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "commodity_reminders" CASCADE;