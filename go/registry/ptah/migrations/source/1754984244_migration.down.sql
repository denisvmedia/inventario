-- Migration rollback
-- Generated on: 2025-08-12T07:37:24Z
-- Direction: DOWN

-- Drop RLS policy area_user_isolation from table 
DROP POLICY IF EXISTS area_user_isolation ON;
-- Drop RLS policy commodity_user_isolation from table 
DROP POLICY IF EXISTS commodity_user_isolation ON;
-- Drop RLS policy export_user_isolation from table 
DROP POLICY IF EXISTS export_user_isolation ON;
-- Drop RLS policy file_user_isolation from table 
DROP POLICY IF EXISTS file_user_isolation ON;
-- Drop RLS policy image_user_isolation from table 
DROP POLICY IF EXISTS image_user_isolation ON;
-- Drop RLS policy invoice_user_isolation from table 
DROP POLICY IF EXISTS invoice_user_isolation ON;
-- Drop RLS policy location_user_isolation from table 
DROP POLICY IF EXISTS location_user_isolation ON;
-- Drop RLS policy manual_user_isolation from table 
DROP POLICY IF EXISTS manual_user_isolation ON;
-- Drop RLS policy restore_operation_user_isolation from table 
DROP POLICY IF EXISTS restore_operation_user_isolation ON;
-- Drop RLS policy restore_step_user_isolation from table 
DROP POLICY IF EXISTS restore_step_user_isolation ON;
-- Drop RLS policy user_user_isolation from table 
DROP POLICY IF EXISTS user_user_isolation ON;
-- NOTE: RLS policies were removed from table  - verify if RLS should be disabled --
-- Remove columns from table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column areas.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column commodities.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column files.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column restore_operations.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column exports.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column images.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column invoices.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column locations.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column manuals.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column restore_steps.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN id CASCADE;
-- WARNING: Dropping column users.id with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column users.user_id with CASCADE - This will delete data and dependent objects! --;