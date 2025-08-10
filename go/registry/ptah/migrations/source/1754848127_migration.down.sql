-- Migration rollback
-- Generated on: 2025-08-10T17:48:47Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_areas_tenant_id;
DROP INDEX IF EXISTS idx_areas_tenant_location;
DROP INDEX IF EXISTS idx_commodities_tenant_area;
DROP INDEX IF EXISTS idx_commodities_tenant_id;
DROP INDEX IF EXISTS idx_commodities_tenant_status;
DROP INDEX IF EXISTS idx_exports_tenant_id;
DROP INDEX IF EXISTS idx_exports_tenant_status;
DROP INDEX IF EXISTS idx_exports_tenant_type;
DROP INDEX IF EXISTS idx_files_tenant_id;
DROP INDEX IF EXISTS idx_files_tenant_linked_entity;
DROP INDEX IF EXISTS idx_files_tenant_type;
DROP INDEX IF EXISTS idx_images_tenant_commodity;
DROP INDEX IF EXISTS idx_images_tenant_id;
DROP INDEX IF EXISTS idx_invoices_tenant_commodity;
DROP INDEX IF EXISTS idx_invoices_tenant_id;
DROP INDEX IF EXISTS idx_locations_tenant_id;
DROP INDEX IF EXISTS idx_manuals_tenant_commodity;
DROP INDEX IF EXISTS idx_manuals_tenant_id;
DROP INDEX IF EXISTS idx_restore_operations_tenant_export;
DROP INDEX IF EXISTS idx_restore_operations_tenant_id;
DROP INDEX IF EXISTS idx_restore_operations_tenant_status;
DROP INDEX IF EXISTS idx_restore_steps_tenant_id;
DROP INDEX IF EXISTS idx_restore_steps_tenant_operation;
DROP INDEX IF EXISTS idx_restore_steps_tenant_result;
DROP INDEX IF EXISTS tenants_domain_idx;
DROP INDEX IF EXISTS tenants_slug_idx;
DROP INDEX IF EXISTS tenants_status_idx;
DROP INDEX IF EXISTS users_active_idx;
DROP INDEX IF EXISTS users_role_idx;
DROP INDEX IF EXISTS users_tenant_email_idx;
DROP INDEX IF EXISTS users_tenant_idx;
-- Drop RLS policy area_tenant_isolation from table 
DROP POLICY IF EXISTS area_tenant_isolation ON;
-- Drop RLS policy commodity_tenant_isolation from table 
DROP POLICY IF EXISTS commodity_tenant_isolation ON;
-- Drop RLS policy export_tenant_isolation from table 
DROP POLICY IF EXISTS export_tenant_isolation ON;
-- Drop RLS policy file_tenant_isolation from table 
DROP POLICY IF EXISTS file_tenant_isolation ON;
-- Drop RLS policy image_tenant_isolation from table 
DROP POLICY IF EXISTS image_tenant_isolation ON;
-- Drop RLS policy invoice_tenant_isolation from table 
DROP POLICY IF EXISTS invoice_tenant_isolation ON;
-- Drop RLS policy location_tenant_isolation from table 
DROP POLICY IF EXISTS location_tenant_isolation ON;
-- Drop RLS policy manual_tenant_isolation from table 
DROP POLICY IF EXISTS manual_tenant_isolation ON;
-- Drop RLS policy restore_operation_tenant_isolation from table 
DROP POLICY IF EXISTS restore_operation_tenant_isolation ON;
-- Drop RLS policy restore_step_tenant_isolation from table 
DROP POLICY IF EXISTS restore_step_tenant_isolation ON;
-- Drop RLS policy user_tenant_isolation from table 
DROP POLICY IF EXISTS user_tenant_isolation ON;
-- NOTE: RLS policies were removed from table  - verify if RLS should be disabled --
-- Remove columns from table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column invoices.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column files.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column exports.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column areas.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column restore_operations.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column restore_steps.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column locations.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column commodities.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column images.tenant_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN tenant_id CASCADE;
-- WARNING: Dropping column manuals.tenant_id with CASCADE - This will delete data and dependent objects! --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS tenants CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS users CASCADE;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS get_current_tenant_id;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS set_tenant_context;
-- WARNING: Ensure no other objects depend on this role
DROP ROLE IF EXISTS inventario_app;