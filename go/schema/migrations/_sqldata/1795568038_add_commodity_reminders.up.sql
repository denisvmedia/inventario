-- Migration generated from schema differences
-- Generated on: 2026-11-25T01:53:58+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: commodity_reminders --
CREATE TABLE "commodity_reminders" (
  "commodity_id" TEXT NOT NULL,
  "title" TEXT NOT NULL,
  "due_on" TEXT NOT NULL,
  "remind_days_before" JSONB NOT NULL,
  "repeat_unit" TEXT NOT NULL DEFAULT 'none',
  "repeat_interval" INTEGER NOT NULL DEFAULT 1,
  "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: commodity_reminder_sends --
CREATE TABLE "commodity_reminder_sends" (
  "reminder_id" TEXT NOT NULL,
  "occurrence_on" TEXT NOT NULL,
  "lead_days" INTEGER NOT NULL,
  "sent_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "group_id" TEXT NOT NULL,
  "created_by_user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for commodity_reminder_sends table
ALTER TABLE "commodity_reminder_sends" ENABLE ROW LEVEL SECURITY;
-- Enable RLS for commodity_reminders table
ALTER TABLE "commodity_reminders" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to record reminder emails across all groups
DROP POLICY IF EXISTS "commodity_reminder_send_background_worker_access" ON "commodity_reminder_sends";
CREATE POLICY "commodity_reminder_send_background_worker_access" ON "commodity_reminder_sends" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures commodity reminder sends are accessible only by their tenant and group
DROP POLICY IF EXISTS "commodity_reminder_send_isolation" ON "commodity_reminder_sends";
CREATE POLICY "commodity_reminder_send_isolation" ON "commodity_reminder_sends" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to read commodity reminders across all groups
DROP POLICY IF EXISTS "commodity_reminder_background_worker_access" ON "commodity_reminders";
CREATE POLICY "commodity_reminder_background_worker_access" ON "commodity_reminders" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures commodity reminders can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS "commodity_reminder_isolation" ON "commodity_reminders";
CREATE POLICY "commodity_reminder_isolation" ON "commodity_reminders" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS "idx_commodity_reminder_sends_group_id" ON "commodity_reminder_sends" ("group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_reminder_sends_key" ON "commodity_reminder_sends" ("reminder_id", "occurrence_on", "lead_days");
CREATE INDEX IF NOT EXISTS "idx_commodity_reminder_sends_tenant_id" ON "commodity_reminder_sends" ("tenant_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_reminders_commodity" ON "commodity_reminders" ("commodity_id");
CREATE INDEX IF NOT EXISTS "idx_commodity_reminders_tenant_group" ON "commodity_reminders" ("tenant_id", "group_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_commodity_reminders_uuid" ON "commodity_reminders" ("uuid");
-- ALTER statements: --
ALTER TABLE "commodity_reminders" ADD CONSTRAINT "fk_commodity_reminder_commodity" FOREIGN KEY ("commodity_id") REFERENCES "commodities"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "commodity_reminders" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_reminders" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodity_reminders" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");
-- ALTER statements: --
ALTER TABLE "commodity_reminder_sends" ADD CONSTRAINT "fk_commodity_reminder_send_reminder" FOREIGN KEY ("reminder_id") REFERENCES "commodity_reminders"("id") ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE "commodity_reminder_sends" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "commodity_reminder_sends" ADD CONSTRAINT "fk_entity_group" FOREIGN KEY ("group_id") REFERENCES "location_groups"("id");
-- ALTER statements: --
ALTER TABLE "commodity_reminder_sends" ADD CONSTRAINT "fk_entity_created_by" FOREIGN KEY ("created_by_user_id") REFERENCES "users"("id");