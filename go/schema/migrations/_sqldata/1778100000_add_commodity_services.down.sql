-- Migration rollback
-- Generated on: 2026-05-05T16:00:00+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_commodity_services_active;
DROP INDEX IF EXISTS idx_commodity_services_commodity;
DROP INDEX IF EXISTS idx_commodity_services_due;
DROP INDEX IF EXISTS idx_commodity_services_tenant_group;
DROP INDEX IF EXISTS idx_commodity_services_tenant_id;
DROP INDEX IF EXISTS idx_commodity_services_uuid;
-- Drop RLS policy commodity_service_background_worker_access from table commodity_services
DROP POLICY IF EXISTS commodity_service_background_worker_access ON commodity_services;
-- Drop RLS policy commodity_service_isolation from table commodity_services
DROP POLICY IF EXISTS commodity_service_isolation ON commodity_services;
-- NOTE: RLS policies were removed from table commodity_services - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS commodity_services CASCADE;
