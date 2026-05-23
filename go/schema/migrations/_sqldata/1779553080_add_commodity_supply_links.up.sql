-- Migration generated from schema differences
-- Generated on: 2026-05-17T10:25:24Z
-- Direction: UP

-- POSTGRES TABLE: commodity_supply_links --
CREATE TABLE commodity_supply_links (
  commodity_id TEXT NOT NULL,
  label TEXT NOT NULL,
  url TEXT NOT NULL,
  notes TEXT,
  sort_order INTEGER NOT NULL DEFAULT '0',
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
-- Mirror of the same manual fixup applied to commodity_loans and
-- commodity_services — deleting the parent commodity drops its supply
-- links cleanly (no orphan rows; supply links only exist in the
-- context of their item).
ALTER TABLE commodity_supply_links ADD CONSTRAINT fk_supply_link_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE commodity_supply_links ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE commodity_supply_links ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE commodity_supply_links ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Enable RLS for commodity_supply_links table
ALTER TABLE commodity_supply_links ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all supply links for processing
DROP POLICY IF EXISTS supply_link_background_worker_access ON commodity_supply_links;
CREATE POLICY supply_link_background_worker_access ON commodity_supply_links FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures supply links can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS supply_link_isolation ON commodity_supply_links;
CREATE POLICY supply_link_isolation ON commodity_supply_links FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS idx_supply_links_commodity ON commodity_supply_links (commodity_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_supply_links_tenant_group ON commodity_supply_links (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_supply_links_tenant_id ON commodity_supply_links (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_supply_links_uuid ON commodity_supply_links (uuid);