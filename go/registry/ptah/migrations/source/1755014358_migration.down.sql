-- Migration rollback
-- Generated on: 2025-08-12T15:59:18Z
-- Direction: DOWN

-- ============================================================================
-- CUSTOM INJECTION START: Proper rollback for user isolation migration
-- WARNING: If this migration is re-generated, this custom logic will be lost!
-- ============================================================================

-- Drop RLS policies
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

-- Drop foreign key constraints
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE areas DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE restore_operations DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE manuals DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE locations DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE files DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE exports DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE commodities DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE images DROP CONSTRAINT IF EXISTS fk_entity_user;
ALTER TABLE restore_steps DROP CONSTRAINT IF EXISTS fk_entity_user;

-- Drop user_id columns
ALTER TABLE areas DROP COLUMN IF EXISTS user_id;
ALTER TABLE restore_operations DROP COLUMN IF EXISTS user_id;
ALTER TABLE invoices DROP COLUMN IF EXISTS user_id;
ALTER TABLE manuals DROP COLUMN IF EXISTS user_id;
ALTER TABLE locations DROP COLUMN IF EXISTS user_id;
ALTER TABLE files DROP COLUMN IF EXISTS user_id;
ALTER TABLE exports DROP COLUMN IF EXISTS user_id;
ALTER TABLE commodities DROP COLUMN IF EXISTS user_id;
ALTER TABLE images DROP COLUMN IF EXISTS user_id;
ALTER TABLE restore_steps DROP COLUMN IF EXISTS user_id;
ALTER TABLE users DROP COLUMN IF EXISTS user_id;

-- Remove the default user (optional - might want to keep for data integrity)
-- DELETE FROM users WHERE id = 'default-user-id';

-- Remove the default tenant (optional - might want to keep for data integrity)
-- DELETE FROM tenants WHERE id = 'default-tenant-id';

-- Drop the id column from users table (this will recreate the original state)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pkey;
ALTER TABLE users DROP COLUMN IF EXISTS id;

-- CUSTOM INJECTION END
-- ============================================================================