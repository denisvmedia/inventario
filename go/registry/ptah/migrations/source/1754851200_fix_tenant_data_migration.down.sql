-- Migration rollback for tenant data migration
-- Generated manually on: 2025-08-10T18:40:00Z
-- Direction: DOWN

-- Step 1: Drop all RLS policies
DROP POLICY IF EXISTS area_tenant_isolation ON areas;
DROP POLICY IF EXISTS commodity_tenant_isolation ON commodities;
DROP POLICY IF EXISTS export_tenant_isolation ON exports;
DROP POLICY IF EXISTS file_tenant_isolation ON files;
DROP POLICY IF EXISTS image_tenant_isolation ON images;
DROP POLICY IF EXISTS invoice_tenant_isolation ON invoices;
DROP POLICY IF EXISTS location_tenant_isolation ON locations;
DROP POLICY IF EXISTS manual_tenant_isolation ON manuals;
DROP POLICY IF EXISTS restore_operation_tenant_isolation ON restore_operations;
DROP POLICY IF EXISTS restore_step_tenant_isolation ON restore_steps;
DROP POLICY IF EXISTS user_tenant_isolation ON users;

-- Step 2: Disable RLS on all tables
ALTER TABLE areas DISABLE ROW LEVEL SECURITY;
ALTER TABLE commodities DISABLE ROW LEVEL SECURITY;
ALTER TABLE exports DISABLE ROW LEVEL SECURITY;
ALTER TABLE files DISABLE ROW LEVEL SECURITY;
ALTER TABLE images DISABLE ROW LEVEL SECURITY;
ALTER TABLE invoices DISABLE ROW LEVEL SECURITY;
ALTER TABLE locations DISABLE ROW LEVEL SECURITY;
ALTER TABLE manuals DISABLE ROW LEVEL SECURITY;
ALTER TABLE restore_operations DISABLE ROW LEVEL SECURITY;
ALTER TABLE restore_steps DISABLE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;

-- Step 3: Drop custom functions
DROP FUNCTION IF EXISTS get_current_tenant_id;
DROP FUNCTION IF EXISTS set_tenant_context;

-- Step 4: Drop application role
DROP ROLE IF EXISTS inventario_app;

-- Step 5: Drop all tenant-related indexes
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
DROP INDEX IF EXISTS idx_users_tenant_id;
DROP INDEX IF EXISTS tenants_domain_idx;
DROP INDEX IF EXISTS tenants_slug_idx;
DROP INDEX IF EXISTS users_tenant_email_idx;

-- Step 6: Drop foreign key constraints
ALTER TABLE areas DROP CONSTRAINT IF EXISTS fk_areas_tenant;
ALTER TABLE commodities DROP CONSTRAINT IF EXISTS fk_commodities_tenant;
ALTER TABLE exports DROP CONSTRAINT IF EXISTS fk_exports_tenant;
ALTER TABLE files DROP CONSTRAINT IF EXISTS fk_files_tenant;
ALTER TABLE images DROP CONSTRAINT IF EXISTS fk_images_tenant;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS fk_invoices_tenant;
ALTER TABLE locations DROP CONSTRAINT IF EXISTS fk_locations_tenant;
ALTER TABLE manuals DROP CONSTRAINT IF EXISTS fk_manuals_tenant;
ALTER TABLE restore_operations DROP CONSTRAINT IF EXISTS fk_restore_operations_tenant;
ALTER TABLE restore_steps DROP CONSTRAINT IF EXISTS fk_restore_steps_tenant;

-- Step 7: Drop tenant_id columns from all tables
-- WARNING: This will delete all tenant data!
ALTER TABLE areas DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE commodities DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE exports DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE files DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE images DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE invoices DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE locations DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE manuals DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE restore_operations DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE restore_steps DROP COLUMN IF EXISTS tenant_id CASCADE;

-- Step 8: Drop users table
-- WARNING: This will delete all user data!
DROP TABLE IF EXISTS users CASCADE;

-- Step 9: Drop tenants table
-- WARNING: This will delete all tenant data!
DROP TABLE IF EXISTS tenants CASCADE;
