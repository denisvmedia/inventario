-- Migration rollback
-- Generated on: 2025-08-11T20:05:24Z
-- Direction: DOWN

-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN id CASCADE;
-- WARNING: Dropping column users.id with CASCADE - This will delete data and dependent objects! --
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS get_current_user_id;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS set_user_context;