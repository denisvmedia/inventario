-- Migration rollback
-- Generated on: 2025-08-11T20:32:07Z
-- Direction: DOWN

-- Drop RLS policies for user isolation
DROP POLICY IF EXISTS area_user_isolation ON areas;
DROP POLICY IF EXISTS commodity_user_isolation ON commodities;
DROP POLICY IF EXISTS export_user_isolation ON exports;
DROP POLICY IF EXISTS file_user_isolation ON files;
DROP POLICY IF EXISTS image_user_isolation ON images;
DROP POLICY IF EXISTS invoice_user_isolation ON invoices;
DROP POLICY IF EXISTS location_user_isolation ON locations;
DROP POLICY IF EXISTS manual_user_isolation ON manuals;
DROP POLICY IF EXISTS restore_operation_user_isolation ON restore_operations;
DROP POLICY IF EXISTS restore_step_user_isolation ON restore_steps;
DROP POLICY IF EXISTS user_user_isolation ON users;

-- Drop foreign key constraints for user_id fields
ALTER TABLE commodities DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE exports DROP CONSTRAINT IF EXISTS fk_export_user;
ALTER TABLE images DROP CONSTRAINT IF EXISTS fk_image_user;
ALTER TABLE manuals DROP CONSTRAINT IF EXISTS fk_manual_user;
ALTER TABLE files DROP CONSTRAINT IF EXISTS fk_file_user;
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_user_user;
ALTER TABLE areas DROP CONSTRAINT IF EXISTS fk_area_user;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS fk_invoice_user;
ALTER TABLE locations DROP CONSTRAINT IF EXISTS fk_location_user;
ALTER TABLE restore_operations DROP CONSTRAINT IF EXISTS fk_restore_operation_user;
ALTER TABLE restore_steps DROP CONSTRAINT IF EXISTS fk_restore_step_user;
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column commodities.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column exports.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column images.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column manuals.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column files.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column users.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column areas.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column invoices.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column locations.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column restore_operations.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column restore_steps.user_id with CASCADE - This will delete data and dependent objects! --;