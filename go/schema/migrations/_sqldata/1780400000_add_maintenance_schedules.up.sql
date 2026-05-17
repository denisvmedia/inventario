-- Migration generated from schema differences
-- Generated on: 2026-05-17T09:19:24Z
-- Direction: UP

-- POSTGRES TABLE: maintenance_reminders --
CREATE TABLE maintenance_reminders (
  schedule_id TEXT NOT NULL,
  threshold_days INTEGER NOT NULL,
  sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: maintenance_schedules --
CREATE TABLE maintenance_schedules (
  commodity_id TEXT NOT NULL,
  title TEXT NOT NULL,
  interval_days INTEGER NOT NULL,
  next_due_at TEXT NOT NULL,
  last_done_at TEXT,
  notes TEXT,
  enabled BOOLEAN NOT NULL DEFAULT 'true',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
-- ON DELETE CASCADE is added manually: the Ptah generator does not yet
-- emit on_delete clauses. Cascading the delete keeps reminder rows
-- from outliving their parent schedule (no orphan rows) — and the
-- schedule itself cascades from its parent commodity.
ALTER TABLE maintenance_reminders ADD CONSTRAINT fk_maintenance_reminder_schedule FOREIGN KEY (schedule_id) REFERENCES maintenance_schedules(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE maintenance_reminders ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE maintenance_reminders ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE maintenance_reminders ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- ALTER statements: --
-- ON DELETE CASCADE — drop schedule rows when their commodity is hard-deleted.
ALTER TABLE maintenance_schedules ADD CONSTRAINT fk_maintenance_schedule_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE maintenance_schedules ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE maintenance_schedules ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE maintenance_schedules ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Enable RLS for maintenance_reminders table
ALTER TABLE maintenance_reminders ENABLE ROW LEVEL SECURITY;
-- Enable RLS for maintenance_schedules table
ALTER TABLE maintenance_schedules ENABLE ROW LEVEL SECURITY;
-- Allows background workers to record reminder emissions across all groups
DROP POLICY IF EXISTS maintenance_reminder_background_worker_access ON maintenance_reminders;
CREATE POLICY maintenance_reminder_background_worker_access ON maintenance_reminders FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures maintenance reminders are accessible only by their tenant and group
DROP POLICY IF EXISTS maintenance_reminder_isolation ON maintenance_reminders;
CREATE POLICY maintenance_reminder_isolation ON maintenance_reminders FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
-- Allows background workers to access all maintenance schedules for processing
DROP POLICY IF EXISTS maintenance_schedule_background_worker_access ON maintenance_schedules;
CREATE POLICY maintenance_schedule_background_worker_access ON maintenance_schedules FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures maintenance schedules can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS maintenance_schedule_isolation ON maintenance_schedules;
CREATE POLICY maintenance_schedule_isolation ON maintenance_schedules FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS idx_maintenance_reminders_group_id ON maintenance_reminders (group_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_maintenance_reminders_schedule_threshold ON maintenance_reminders (schedule_id, threshold_days);
CREATE INDEX IF NOT EXISTS idx_maintenance_reminders_tenant_id ON maintenance_reminders (tenant_id);
CREATE INDEX IF NOT EXISTS idx_maintenance_schedules_commodity ON maintenance_schedules (commodity_id, next_due_at);
CREATE INDEX IF NOT EXISTS idx_maintenance_schedules_enabled_due ON maintenance_schedules (next_due_at) WHERE enabled = true;
CREATE INDEX IF NOT EXISTS idx_maintenance_schedules_group_due ON maintenance_schedules (group_id, next_due_at);
CREATE INDEX IF NOT EXISTS idx_maintenance_schedules_tenant_group ON maintenance_schedules (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_maintenance_schedules_tenant_id ON maintenance_schedules (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_maintenance_schedules_uuid ON maintenance_schedules (uuid);