-- Migration rollback
-- Generated on: 2025-09-14T20:32:29+02:00
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
DROP INDEX IF EXISTS idx_settings_tenant_id;
DROP INDEX IF EXISTS idx_settings_tenant_user_name;
DROP INDEX IF EXISTS idx_settings_user_id;
DROP INDEX IF EXISTS idx_thumbnail_jobs_cleanup;
DROP INDEX IF EXISTS idx_thumbnail_jobs_file_id;
DROP INDEX IF EXISTS idx_thumbnail_jobs_status_created;
DROP INDEX IF EXISTS idx_thumbnail_jobs_tenant_id;
DROP INDEX IF EXISTS idx_thumbnail_jobs_user_status;
DROP INDEX IF EXISTS idx_user_concurrency_slots_job_id;
DROP INDEX IF EXISTS idx_user_concurrency_slots_status;
DROP INDEX IF EXISTS idx_user_concurrency_slots_tenant_id;
DROP INDEX IF EXISTS idx_user_concurrency_slots_user_id;
DROP INDEX IF EXISTS idx_user_concurrency_slots_user_status;
DROP INDEX IF EXISTS settings_value_gin_idx;
DROP INDEX IF EXISTS tenants_domain_idx;
DROP INDEX IF EXISTS tenants_slug_idx;
DROP INDEX IF EXISTS tenants_status_idx;
DROP INDEX IF EXISTS users_active_idx;
DROP INDEX IF EXISTS users_role_idx;
DROP INDEX IF EXISTS users_tenant_email_idx;
DROP INDEX IF EXISTS users_tenant_idx;
-- Drop RLS policy area_background_worker_access from table areas
DROP POLICY IF EXISTS area_background_worker_access ON areas;
-- Drop RLS policy area_isolation from table areas
DROP POLICY IF EXISTS area_isolation ON areas;
-- Drop RLS policy commodity_background_worker_access from table commodities
DROP POLICY IF EXISTS commodity_background_worker_access ON commodities;
-- Drop RLS policy commodity_isolation from table commodities
DROP POLICY IF EXISTS commodity_isolation ON commodities;
-- Drop RLS policy export_background_worker_access from table exports
DROP POLICY IF EXISTS export_background_worker_access ON exports;
-- Drop RLS policy export_isolation from table exports
DROP POLICY IF EXISTS export_isolation ON exports;
-- Drop RLS policy file_background_worker_access from table files
DROP POLICY IF EXISTS file_background_worker_access ON files;
-- Drop RLS policy file_isolation from table files
DROP POLICY IF EXISTS file_isolation ON files;
-- Drop RLS policy image_background_worker_access from table images
DROP POLICY IF EXISTS image_background_worker_access ON images;
-- Drop RLS policy image_isolation from table images
DROP POLICY IF EXISTS image_isolation ON images;
-- Drop RLS policy invoice_background_worker_access from table invoices
DROP POLICY IF EXISTS invoice_background_worker_access ON invoices;
-- Drop RLS policy invoice_isolation from table invoices
DROP POLICY IF EXISTS invoice_isolation ON invoices;
-- Drop RLS policy location_background_worker_access from table locations
DROP POLICY IF EXISTS location_background_worker_access ON locations;
-- Drop RLS policy location_isolation from table locations
DROP POLICY IF EXISTS location_isolation ON locations;
-- Drop RLS policy manual_background_worker_access from table manuals
DROP POLICY IF EXISTS manual_background_worker_access ON manuals;
-- Drop RLS policy manual_isolation from table manuals
DROP POLICY IF EXISTS manual_isolation ON manuals;
-- Drop RLS policy restore_operation_background_worker_access from table restore_operations
DROP POLICY IF EXISTS restore_operation_background_worker_access ON restore_operations;
-- Drop RLS policy restore_operation_isolation from table restore_operations
DROP POLICY IF EXISTS restore_operation_isolation ON restore_operations;
-- Drop RLS policy restore_step_background_worker_access from table restore_steps
DROP POLICY IF EXISTS restore_step_background_worker_access ON restore_steps;
-- Drop RLS policy restore_step_isolation from table restore_steps
DROP POLICY IF EXISTS restore_step_isolation ON restore_steps;
-- Drop RLS policy setting_background_worker_access from table settings
DROP POLICY IF EXISTS setting_background_worker_access ON settings;
-- Drop RLS policy setting_isolation from table settings
DROP POLICY IF EXISTS setting_isolation ON settings;
-- Drop RLS policy thumbnail_generation_job_background_worker_access from table thumbnail_generation_jobs
DROP POLICY IF EXISTS thumbnail_generation_job_background_worker_access ON thumbnail_generation_jobs;
-- Drop RLS policy thumbnail_generation_job_isolation from table thumbnail_generation_jobs
DROP POLICY IF EXISTS thumbnail_generation_job_isolation ON thumbnail_generation_jobs;
-- Drop RLS policy user_background_worker_access from table users
DROP POLICY IF EXISTS user_background_worker_access ON users;
-- Drop RLS policy user_concurrency_slot_background_worker_access from table user_concurrency_slots
DROP POLICY IF EXISTS user_concurrency_slot_background_worker_access ON user_concurrency_slots;
-- Drop RLS policy user_concurrency_slot_isolation from table user_concurrency_slots
DROP POLICY IF EXISTS user_concurrency_slot_isolation ON user_concurrency_slots;
-- Drop RLS policy user_isolation from table users
DROP POLICY IF EXISTS user_isolation ON users;
-- NOTE: RLS policies were removed from table restore_operations - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table settings - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table users - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table user_concurrency_slots - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table areas - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table commodities - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table exports - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table images - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table invoices - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table manuals - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table restore_steps - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table thumbnail_generation_jobs - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table files - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table locations - verify if RLS should be disabled --
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
DROP TABLE IF EXISTS thumbnail_generation_jobs CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS user_concurrency_slots CASCADE;
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