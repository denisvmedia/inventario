-- Migration generated from schema differences
-- Generated on: 2026-05-02T17:07:51+02:00
-- Direction: UP

DROP INDEX IF EXISTS idx_images_tenant_commodity;
DROP INDEX IF EXISTS idx_images_tenant_group;
DROP INDEX IF EXISTS idx_images_tenant_id;
DROP INDEX IF EXISTS idx_images_uuid;
DROP INDEX IF EXISTS idx_invoices_tenant_commodity;
DROP INDEX IF EXISTS idx_invoices_tenant_group;
DROP INDEX IF EXISTS idx_invoices_tenant_id;
DROP INDEX IF EXISTS idx_invoices_uuid;
DROP INDEX IF EXISTS idx_manuals_tenant_commodity;
DROP INDEX IF EXISTS idx_manuals_tenant_group;
DROP INDEX IF EXISTS idx_manuals_tenant_id;
DROP INDEX IF EXISTS idx_manuals_uuid;
-- Drop RLS policy image_background_worker_access from table images
DROP POLICY IF EXISTS image_background_worker_access ON images;
-- Drop RLS policy image_isolation from table images
DROP POLICY IF EXISTS image_isolation ON images;
-- Drop RLS policy invoice_background_worker_access from table invoices
DROP POLICY IF EXISTS invoice_background_worker_access ON invoices;
-- Drop RLS policy invoice_isolation from table invoices
DROP POLICY IF EXISTS invoice_isolation ON invoices;
-- Drop RLS policy manual_background_worker_access from table manuals
DROP POLICY IF EXISTS manual_background_worker_access ON manuals;
-- Drop RLS policy manual_isolation from table manuals
DROP POLICY IF EXISTS manual_isolation ON manuals;
-- NOTE: RLS policies were removed from table images - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table invoices - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table manuals - verify if RLS should be disabled --
-- Temporary function to drop constraint 2200_17082_1_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_1_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_1_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_1_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_1_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_1_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_1_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_1_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_1_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_1_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_1_not_null;
-- Clean up executor function for 2200_17082_1_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_1_not_null;
-- Temporary function to drop constraint 2200_17082_3_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_3_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_3_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_3_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_3_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_3_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_3_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_3_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_3_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_3_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_3_not_null;
-- Clean up executor function for 2200_17082_3_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_3_not_null;
-- Temporary function to drop constraint 2200_17068_8_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_8_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_8_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_8_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_8_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_8_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_8_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_8_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_8_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_8_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_8_not_null;
-- Clean up executor function for 2200_17068_8_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_8_not_null;
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
-- Temporary function to drop constraint 2200_17075_10_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_10_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_10_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_10_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_10_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_10_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_10_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_10_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_10_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_10_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_10_not_null;
-- Clean up executor function for 2200_17075_10_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_10_not_null;
-- Temporary function to drop constraint 2200_17075_1_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_1_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_1_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_1_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_1_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_1_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_1_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_1_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_1_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_1_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_1_not_null;
-- Clean up executor function for 2200_17075_1_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_1_not_null;
-- Temporary function to drop constraint fk_entity_tenant
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_entity_tenant() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_entity_tenant'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_entity_tenant');
        RAISE NOTICE 'Dropped constraint fk_entity_tenant from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_entity_tenant not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_entity_tenant
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_entity_tenant() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_entity_tenant();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_entity_tenant
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_entity_tenant;
-- Clean up executor function for fk_entity_tenant
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_entity_tenant;
-- Temporary function to drop constraint 2200_17082_5_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_5_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_5_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_5_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_5_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_5_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_5_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_5_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_5_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_5_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_5_not_null;
-- Clean up executor function for 2200_17082_5_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_5_not_null;
-- Temporary function to drop constraint 2200_17082_6_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_6_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_6_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_6_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_6_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_6_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_6_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_6_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_6_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_6_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_6_not_null;
-- Clean up executor function for 2200_17082_6_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_6_not_null;
-- Temporary function to drop constraint 2200_17068_6_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_6_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_6_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_6_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_6_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_6_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_6_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_6_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_6_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_6_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_6_not_null;
-- Clean up executor function for 2200_17068_6_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_6_not_null;
-- Temporary function to drop constraint 2200_17068_9_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_9_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_9_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_9_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_9_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_9_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_9_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_9_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_9_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_9_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_9_not_null;
-- Clean up executor function for 2200_17068_9_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_9_not_null;
-- Temporary function to drop constraint fk_image_commodity
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_image_commodity() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_image_commodity'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_image_commodity');
        RAISE NOTICE 'Dropped constraint fk_image_commodity from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_image_commodity not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_image_commodity
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_image_commodity() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_image_commodity();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_image_commodity
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_image_commodity;
-- Clean up executor function for fk_image_commodity
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_image_commodity;
-- Temporary function to drop constraint images_pkey
CREATE OR REPLACE FUNCTION ptah_drop_constraint_images_pkey() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'images_pkey'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'images_pkey');
        RAISE NOTICE 'Dropped constraint images_pkey from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint images_pkey not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for images_pkey
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_images_pkey() RETURNS VOID AS $$
SELECT ptah_drop_constraint_images_pkey();
$$
LANGUAGE sql;
-- Clean up temporary function for images_pkey
DROP FUNCTION IF EXISTS ptah_drop_constraint_images_pkey;
-- Clean up executor function for images_pkey
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_images_pkey;
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
-- Temporary function to drop constraint fk_invoice_commodity
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_invoice_commodity() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_invoice_commodity'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_invoice_commodity');
        RAISE NOTICE 'Dropped constraint fk_invoice_commodity from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_invoice_commodity not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_invoice_commodity
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_invoice_commodity() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_invoice_commodity();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_invoice_commodity
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_invoice_commodity;
-- Clean up executor function for fk_invoice_commodity
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_invoice_commodity;
-- Temporary function to drop constraint 2200_17082_2_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_2_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_2_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_2_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_2_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_2_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_2_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_2_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_2_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_2_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_2_not_null;
-- Clean up executor function for 2200_17082_2_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_2_not_null;
-- Temporary function to drop constraint 2200_17082_7_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_7_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_7_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_7_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_7_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_7_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_7_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_7_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_7_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_7_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_7_not_null;
-- Clean up executor function for 2200_17082_7_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_7_not_null;
-- Temporary function to drop constraint 2200_17075_3_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_3_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_3_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_3_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_3_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_3_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_3_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_3_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_3_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_3_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_3_not_null;
-- Clean up executor function for 2200_17075_3_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_3_not_null;
-- Temporary function to drop constraint 2200_17075_6_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_6_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_6_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_6_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_6_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_6_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_6_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_6_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_6_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_6_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_6_not_null;
-- Clean up executor function for 2200_17075_6_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_6_not_null;
-- Temporary function to drop constraint 2200_17075_8_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_8_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_8_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_8_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_8_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_8_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_8_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_8_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_8_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_8_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_8_not_null;
-- Clean up executor function for 2200_17075_8_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_8_not_null;
-- Temporary function to drop constraint 2200_17075_9_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_9_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_9_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_9_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_9_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_9_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_9_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_9_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_9_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_9_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_9_not_null;
-- Clean up executor function for 2200_17075_9_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_9_not_null;
-- Temporary function to drop constraint invoices_pkey
CREATE OR REPLACE FUNCTION ptah_drop_constraint_invoices_pkey() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'invoices_pkey'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'invoices_pkey');
        RAISE NOTICE 'Dropped constraint invoices_pkey from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint invoices_pkey not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for invoices_pkey
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_invoices_pkey() RETURNS VOID AS $$
SELECT ptah_drop_constraint_invoices_pkey();
$$
LANGUAGE sql;
-- Clean up temporary function for invoices_pkey
DROP FUNCTION IF EXISTS ptah_drop_constraint_invoices_pkey;
-- Clean up executor function for invoices_pkey
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_invoices_pkey;
-- Temporary function to drop constraint 2200_17082_4_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_4_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_4_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_4_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_4_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_4_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_4_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_4_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_4_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_4_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_4_not_null;
-- Clean up executor function for 2200_17082_4_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_4_not_null;
-- Temporary function to drop constraint fk_entity_tenant
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_entity_tenant() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_entity_tenant'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_entity_tenant');
        RAISE NOTICE 'Dropped constraint fk_entity_tenant from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_entity_tenant not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_entity_tenant
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_entity_tenant() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_entity_tenant();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_entity_tenant
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_entity_tenant;
-- Clean up executor function for fk_entity_tenant
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_entity_tenant;
-- Temporary function to drop constraint 2200_17068_10_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_10_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_10_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_10_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_10_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_10_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_10_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_10_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_10_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_10_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_10_not_null;
-- Clean up executor function for 2200_17068_10_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_10_not_null;
-- Temporary function to drop constraint 2200_17068_5_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_5_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_5_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_5_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_5_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_5_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_5_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_5_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_5_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_5_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_5_not_null;
-- Clean up executor function for 2200_17068_5_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_5_not_null;
-- Temporary function to drop constraint 2200_17068_7_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_7_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_7_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_7_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_7_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_7_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_7_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_7_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_7_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_7_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_7_not_null;
-- Clean up executor function for 2200_17068_7_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_7_not_null;
-- Temporary function to drop constraint 2200_17075_5_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_5_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_5_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_5_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_5_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_5_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_5_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_5_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_5_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_5_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_5_not_null;
-- Clean up executor function for 2200_17075_5_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_5_not_null;
-- Temporary function to drop constraint 2200_17075_7_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_7_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_7_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_7_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_7_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_7_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_7_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_7_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_7_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_7_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_7_not_null;
-- Clean up executor function for 2200_17075_7_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_7_not_null;
-- Temporary function to drop constraint 2200_17082_8_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_8_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_8_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_8_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_8_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_8_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_8_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_8_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_8_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_8_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_8_not_null;
-- Clean up executor function for 2200_17082_8_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_8_not_null;
-- Temporary function to drop constraint fk_entity_tenant
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_entity_tenant() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_entity_tenant'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_entity_tenant');
        RAISE NOTICE 'Dropped constraint fk_entity_tenant from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_entity_tenant not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_entity_tenant
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_entity_tenant() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_entity_tenant();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_entity_tenant
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_entity_tenant;
-- Clean up executor function for fk_entity_tenant
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_entity_tenant;
-- Temporary function to drop constraint fk_invoice_group
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_invoice_group() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_invoice_group'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_invoice_group');
        RAISE NOTICE 'Dropped constraint fk_invoice_group from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_invoice_group not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_invoice_group
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_invoice_group() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_invoice_group();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_invoice_group
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_invoice_group;
-- Clean up executor function for fk_invoice_group
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_invoice_group;
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
-- Temporary function to drop constraint fk_manual_commodity
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_manual_commodity() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_manual_commodity'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_manual_commodity');
        RAISE NOTICE 'Dropped constraint fk_manual_commodity from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_manual_commodity not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_manual_commodity
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_manual_commodity() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_manual_commodity();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_manual_commodity
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_manual_commodity;
-- Clean up executor function for fk_manual_commodity
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_manual_commodity;
-- Temporary function to drop constraint 2200_17068_3_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_3_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_3_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_3_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_3_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_3_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_3_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_3_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_3_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_3_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_3_not_null;
-- Clean up executor function for 2200_17068_3_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_3_not_null;
-- Temporary function to drop constraint fk_image_group
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_image_group() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_image_group'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_image_group');
        RAISE NOTICE 'Dropped constraint fk_image_group from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_image_group not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_image_group
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_image_group() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_image_group();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_image_group
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_image_group;
-- Clean up executor function for fk_image_group
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_image_group;
-- Temporary function to drop constraint 2200_17082_9_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_9_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_9_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_9_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_9_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_9_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_9_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_9_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_9_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_9_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_9_not_null;
-- Clean up executor function for 2200_17082_9_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_9_not_null;
-- Temporary function to drop constraint fk_manual_group
CREATE OR REPLACE FUNCTION ptah_drop_constraint_fk_manual_group() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_manual_group'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_manual_group');
        RAISE NOTICE 'Dropped constraint fk_manual_group from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_manual_group not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for fk_manual_group
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_fk_manual_group() RETURNS VOID AS $$
SELECT ptah_drop_constraint_fk_manual_group();
$$
LANGUAGE sql;
-- Clean up temporary function for fk_manual_group
DROP FUNCTION IF EXISTS ptah_drop_constraint_fk_manual_group;
-- Clean up executor function for fk_manual_group
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_fk_manual_group;
-- Temporary function to drop constraint manuals_pkey
CREATE OR REPLACE FUNCTION ptah_drop_constraint_manuals_pkey() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'manuals_pkey'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'manuals_pkey');
        RAISE NOTICE 'Dropped constraint manuals_pkey from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint manuals_pkey not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for manuals_pkey
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_manuals_pkey() RETURNS VOID AS $$
SELECT ptah_drop_constraint_manuals_pkey();
$$
LANGUAGE sql;
-- Clean up temporary function for manuals_pkey
DROP FUNCTION IF EXISTS ptah_drop_constraint_manuals_pkey;
-- Clean up executor function for manuals_pkey
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_manuals_pkey;
-- Temporary function to drop constraint 2200_17068_1_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_1_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_1_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_1_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_1_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_1_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_1_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_1_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_1_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_1_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_1_not_null;
-- Clean up executor function for 2200_17068_1_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_1_not_null;
-- Temporary function to drop constraint 2200_17068_2_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_2_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_2_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_2_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_2_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_2_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_2_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_2_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_2_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_2_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_2_not_null;
-- Clean up executor function for 2200_17068_2_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_2_not_null;
-- Temporary function to drop constraint 2200_17068_4_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17068_4_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17068_4_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17068_4_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17068_4_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17068_4_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17068_4_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17068_4_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17068_4_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17068_4_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17068_4_not_null;
-- Clean up executor function for 2200_17068_4_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17068_4_not_null;
-- Temporary function to drop constraint 2200_17075_2_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_2_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_2_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_2_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_2_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_2_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_2_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_2_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_2_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_2_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_2_not_null;
-- Clean up executor function for 2200_17075_2_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_2_not_null;
-- Temporary function to drop constraint 2200_17082_10_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17082_10_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17082_10_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17082_10_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17082_10_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17082_10_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17082_10_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17082_10_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17082_10_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17082_10_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17082_10_not_null;
-- Clean up executor function for 2200_17082_10_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17082_10_not_null;
-- Temporary function to drop constraint 2200_17075_4_not_null
CREATE OR REPLACE FUNCTION ptah_drop_constraint_2200_17075_4_not_null() RETURNS VOID AS $$
DECLARE
    target_table TEXT;
BEGIN
    -- Find the table that contains this constraint
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = '2200_17075_4_not_null'
      AND table_schema = current_schema()
    LIMIT 1;

    -- Drop the constraint if found
    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, '2200_17075_4_not_null');
        RAISE NOTICE 'Dropped constraint 2200_17075_4_not_null from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint 2200_17075_4_not_null not found in current schema';
    END IF;
END;
$$
LANGUAGE plpgsql;
-- Execute constraint removal for 2200_17075_4_not_null
CREATE OR REPLACE FUNCTION ptah_exec_ptah_drop_constraint_2200_17075_4_not_null() RETURNS VOID AS $$
SELECT ptah_drop_constraint_2200_17075_4_not_null();
$$
LANGUAGE sql;
-- Clean up temporary function for 2200_17075_4_not_null
DROP FUNCTION IF EXISTS ptah_drop_constraint_2200_17075_4_not_null;
-- Clean up executor function for 2200_17075_4_not_null
DROP FUNCTION IF EXISTS ptah_exec_ptah_drop_constraint_2200_17075_4_not_null;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS images CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS invoices CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS manuals CASCADE;