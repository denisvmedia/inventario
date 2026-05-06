-- Migration generated from schema differences
-- Generated on: 2026-05-05T16:00:00+02:00
-- Direction: UP

-- POSTGRES TABLE: commodity_services --
CREATE TABLE commodity_services (
  commodity_id TEXT NOT NULL,
  provider_name TEXT NOT NULL,
  provider_contact TEXT,
  reason TEXT,
  sent_at TEXT NOT NULL,
  expected_return_at TEXT,
  returned_at TEXT,
  cost_amount DECIMAL(14,2),
  cost_currency TEXT,
  reminder_sent_overdue BOOLEAN NOT NULL DEFAULT 'false',
  reminder_sent_due_soon BOOLEAN NOT NULL DEFAULT 'false',
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
-- emit delete behaviour even when the model carries on_delete="CASCADE".
-- Mirror of the manual fixup applied to commodity_loans (#1506) and
-- commodity_events (#1505).
ALTER TABLE commodity_services ADD CONSTRAINT fk_commodity_service_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE commodity_services ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE commodity_services ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE commodity_services ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Enable RLS for commodity_services table
ALTER TABLE commodity_services ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all commodity services for processing
DROP POLICY IF EXISTS commodity_service_background_worker_access ON commodity_services;
CREATE POLICY commodity_service_background_worker_access ON commodity_services FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures commodity services can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS commodity_service_isolation ON commodity_services;
CREATE POLICY commodity_service_isolation ON commodity_services FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS idx_commodity_services_active ON commodity_services (group_id, expected_return_at) WHERE returned_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_commodity_services_commodity ON commodity_services (commodity_id, sent_at);
CREATE INDEX IF NOT EXISTS idx_commodity_services_due ON commodity_services (expected_return_at) WHERE returned_at IS NULL AND expected_return_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_commodity_services_tenant_group ON commodity_services (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_commodity_services_tenant_id ON commodity_services (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_commodity_services_uuid ON commodity_services (uuid);
