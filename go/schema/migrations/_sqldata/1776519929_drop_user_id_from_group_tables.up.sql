-- Migration generated from schema differences
-- Generated on: 2026-04-18T15:45:29+02:00
-- Direction: UP

-- Remove columns from table: group_invites --
-- ALTER statements: --
ALTER TABLE group_invites DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column group_invites.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: group_memberships --
-- ALTER statements: --
ALTER TABLE group_memberships DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column group_memberships.user_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: location_groups --
-- ALTER statements: --
ALTER TABLE location_groups DROP COLUMN user_id CASCADE;
-- WARNING: Dropping column location_groups.user_id with CASCADE - This will delete data and dependent objects! --
-- Temporary function to drop constraint fk_entity_user
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_entity_user() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_entity_user'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_entity_user');
        RAISE NOTICE 'Dropped constraint fk_entity_user from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_entity_user not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_entity_user
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_entity_user() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_entity_user();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_entity_user
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_entity_user;
-- Clean up executor function for fk_entity_user
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_entity_user;
-- Temporary function to drop constraint fk_entity_user
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_entity_user() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_entity_user'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_entity_user');
        RAISE NOTICE 'Dropped constraint fk_entity_user from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_entity_user not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_entity_user
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_entity_user() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_entity_user();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_entity_user
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_entity_user;
-- Clean up executor function for fk_entity_user
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_entity_user;
-- Temporary function to drop constraint fk_entity_user
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_entity_user() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_entity_user'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_entity_user');
        RAISE NOTICE 'Dropped constraint fk_entity_user from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_entity_user not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_entity_user
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_entity_user() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_entity_user();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_entity_user
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_entity_user;
-- Clean up executor function for fk_entity_user
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_entity_user;