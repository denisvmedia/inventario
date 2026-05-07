-- Migration generated from schema differences
-- Generated on: 2026-05-06T19:59:07Z
-- Direction: UP

-- POSTGRES TABLE: warranty_reminders --
CREATE TABLE warranty_reminders (
  commodity_id TEXT NOT NULL,
  threshold_days INTEGER NOT NULL,
  sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
-- ON DELETE CASCADE is added manually: the Ptah generator does not yet
-- emit on_delete clauses. Cascading the delete keeps idempotency rows
-- from outliving their parent commodity (no orphan rows).
ALTER TABLE warranty_reminders ADD CONSTRAINT fk_warranty_reminder_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE warranty_reminders ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE warranty_reminders ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE warranty_reminders ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN warranty_expires_at TEXT;
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN warranty_notes TEXT;
-- Enable RLS for warranty_reminders table
ALTER TABLE warranty_reminders ENABLE ROW LEVEL SECURITY;
-- Allows background workers to record reminder emissions across all groups
DROP POLICY IF EXISTS warranty_reminder_background_worker_access ON warranty_reminders;
CREATE POLICY warranty_reminder_background_worker_access ON warranty_reminders FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures warranty reminders are accessible only by their tenant and group
DROP POLICY IF EXISTS warranty_reminder_isolation ON warranty_reminders;
CREATE POLICY warranty_reminder_isolation ON warranty_reminders FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS commodities_warranty_expires_at_idx ON commodities (warranty_expires_at) WHERE warranty_expires_at IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_warranty_reminders_commodity_threshold ON warranty_reminders (commodity_id, threshold_days);
CREATE INDEX IF NOT EXISTS idx_warranty_reminders_group_id ON warranty_reminders (group_id);
CREATE INDEX IF NOT EXISTS idx_warranty_reminders_tenant_id ON warranty_reminders (tenant_id);