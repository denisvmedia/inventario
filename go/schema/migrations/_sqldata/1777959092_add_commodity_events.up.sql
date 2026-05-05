-- Migration generated from schema differences
-- Generated on: 2026-05-05T07:31:32+02:00
-- Direction: UP

-- POSTGRES TABLE: commodity_events --
CREATE TABLE commodity_events (
  commodity_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  before JSONB,
  after JSONB,
  note TEXT,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
-- ON DELETE CASCADE is added manually: the Ptah generator does not yet emit
-- delete behaviour even when the model carries on_delete="CASCADE". Keeping
-- it explicit here so the audit timeline goes away with its parent commodity
-- (the detail page won't exist anymore either) without leaving orphan rows
-- the GroupPurger would otherwise have to chase.
ALTER TABLE commodity_events ADD CONSTRAINT fk_commodity_event_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE commodity_events ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE commodity_events ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE commodity_events ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Enable RLS for commodity_events table
ALTER TABLE commodity_events ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all commodity events for processing
DROP POLICY IF EXISTS commodity_event_background_worker_access ON commodity_events;
CREATE POLICY commodity_event_background_worker_access ON commodity_events FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures commodity events can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS commodity_event_isolation ON commodity_events;
CREATE POLICY commodity_event_isolation ON commodity_events FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS commodity_events_kind_idx ON commodity_events (commodity_id, kind);
CREATE INDEX IF NOT EXISTS commodity_events_lookup ON commodity_events (group_id, commodity_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_commodity_events_tenant_group ON commodity_events (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_commodity_events_tenant_id ON commodity_events (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_commodity_events_uuid ON commodity_events (uuid);