-- Migration rollback
-- Generated on: 2026-05-17T10:25:24Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_supply_links_commodity;
DROP INDEX IF EXISTS idx_supply_links_tenant_group;
DROP INDEX IF EXISTS idx_supply_links_tenant_id;
DROP INDEX IF EXISTS idx_supply_links_uuid;
-- Drop RLS policy supply_link_background_worker_access from table commodity_supply_links
DROP POLICY IF EXISTS supply_link_background_worker_access ON commodity_supply_links;
-- Drop RLS policy supply_link_isolation from table commodity_supply_links
DROP POLICY IF EXISTS supply_link_isolation ON commodity_supply_links;
-- NOTE: RLS policies were removed from table commodity_supply_links - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS commodity_supply_links CASCADE;