-- Migration rollback
-- Generated on: 2025-08-18T23:59:51+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS commodities_active_idx;
DROP INDEX IF EXISTS commodities_draft_idx;
DROP INDEX IF EXISTS commodities_extra_serial_numbers_gin_idx;
DROP INDEX IF EXISTS commodities_name_trgm_idx;
DROP INDEX IF EXISTS commodities_part_numbers_gin_idx;
DROP INDEX IF EXISTS commodities_short_name_trgm_idx;
DROP INDEX IF EXISTS commodities_tags_gin_idx;
DROP INDEX IF EXISTS commodities_urls_gin_idx;
DROP INDEX IF EXISTS files_linked_entity_idx;
DROP INDEX IF EXISTS files_linked_entity_meta_idx;
DROP INDEX IF EXISTS files_path_trgm_idx;
DROP INDEX IF EXISTS files_tags_gin_idx;
DROP INDEX IF EXISTS files_title_trgm_idx;
DROP INDEX IF EXISTS files_type_created_idx;
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
-- Drop RLS policy area_user_isolation from table 
DROP POLICY IF EXISTS area_user_isolation ON;
-- Drop RLS policy commodity_tenant_isolation from table 
DROP POLICY IF EXISTS commodity_tenant_isolation ON;
-- Drop RLS policy commodity_user_isolation from table 
DROP POLICY IF EXISTS commodity_user_isolation ON;
-- Drop RLS policy export_tenant_isolation from table 
DROP POLICY IF EXISTS export_tenant_isolation ON;
-- Drop RLS policy export_user_isolation from table 
DROP POLICY IF EXISTS export_user_isolation ON;
-- Drop RLS policy file_tenant_isolation from table 
DROP POLICY IF EXISTS file_tenant_isolation ON;
-- Drop RLS policy file_user_isolation from table 
DROP POLICY IF EXISTS file_user_isolation ON;
-- Drop RLS policy image_tenant_isolation from table 
DROP POLICY IF EXISTS image_tenant_isolation ON;
-- Drop RLS policy image_user_isolation from table 
DROP POLICY IF EXISTS image_user_isolation ON;
-- Drop RLS policy invoice_tenant_isolation from table 
DROP POLICY IF EXISTS invoice_tenant_isolation ON;
-- Drop RLS policy invoice_user_isolation from table 
DROP POLICY IF EXISTS invoice_user_isolation ON;
-- Drop RLS policy location_tenant_isolation from table 
DROP POLICY IF EXISTS location_tenant_isolation ON;
-- Drop RLS policy location_user_isolation from table 
DROP POLICY IF EXISTS location_user_isolation ON;
-- Drop RLS policy manual_tenant_isolation from table 
DROP POLICY IF EXISTS manual_tenant_isolation ON;
-- Drop RLS policy manual_user_isolation from table 
DROP POLICY IF EXISTS manual_user_isolation ON;
-- Drop RLS policy restore_operation_tenant_isolation from table 
DROP POLICY IF EXISTS restore_operation_tenant_isolation ON;
-- Drop RLS policy restore_operation_user_isolation from table 
DROP POLICY IF EXISTS restore_operation_user_isolation ON;
-- Drop RLS policy restore_step_tenant_isolation from table 
DROP POLICY IF EXISTS restore_step_tenant_isolation ON;
-- Drop RLS policy restore_step_user_isolation from table 
DROP POLICY IF EXISTS restore_step_user_isolation ON;
-- Drop RLS policy user_tenant_isolation from table 
DROP POLICY IF EXISTS user_tenant_isolation ON;
-- Drop RLS policy user_user_isolation from table 
DROP POLICY IF EXISTS user_user_isolation ON;
-- NOTE: RLS policies were removed from table  - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS areas CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS commodities CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS exports CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS files CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS images CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS invoices CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS locations CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS manuals CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS restore_operations CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS restore_steps CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS settings CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS tenants CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS users CASCADE;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS get_current_tenant_id;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS get_current_user_id;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS set_tenant_context;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS set_user_context;
-- WARNING: Ensure no other objects depend on this role
DROP ROLE IF EXISTS inventario_app;