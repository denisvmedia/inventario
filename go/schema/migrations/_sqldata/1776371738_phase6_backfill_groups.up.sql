-- Phase 6: Data backfill + column rename + role removal
-- This is a hand-written migration (not Ptah-generated).
-- It must run AFTER phase5_add_nullable_group_id and BEFORE the Ptah-generated phase7 migration.

-- Step 1: Create a default location group for every user. We don't filter to
-- users with existing data because every user needs a default group going
-- forward — any future create will require a non-null group_id regardless of
-- whether the user had pre-existing data.
INSERT INTO location_groups (id, uuid, tenant_id, user_id, slug, name, status, created_by, created_at, updated_at)
SELECT DISTINCT ON (u.tenant_id, u.id)
    gen_random_uuid()::text,
    gen_random_uuid()::text,
    u.tenant_id,
    u.id,
    encode(gen_random_bytes(16), 'hex'),
    u.name || '''s Group',
    'active',
    u.id,
    NOW(), NOW()
FROM users u;

-- Step 2: Create admin memberships for each user in their default group
INSERT INTO group_memberships (id, uuid, tenant_id, user_id, group_id, member_user_id, role, joined_at)
SELECT
    gen_random_uuid()::text,
    gen_random_uuid()::text,
    lg.tenant_id,
    lg.user_id,
    lg.id,
    lg.created_by,
    'admin',
    NOW()
FROM location_groups lg;

-- Step 3: Backfill group_id in all data tables (BEFORE rename — queries reference user_id)
UPDATE locations    SET group_id = (SELECT id FROM location_groups WHERE created_by = locations.user_id    AND tenant_id = locations.tenant_id    LIMIT 1);
UPDATE areas        SET group_id = (SELECT id FROM location_groups WHERE created_by = areas.user_id        AND tenant_id = areas.tenant_id        LIMIT 1);
UPDATE commodities  SET group_id = (SELECT id FROM location_groups WHERE created_by = commodities.user_id  AND tenant_id = commodities.tenant_id  LIMIT 1);
UPDATE files        SET group_id = (SELECT id FROM location_groups WHERE created_by = files.user_id        AND tenant_id = files.tenant_id        LIMIT 1);
UPDATE exports      SET group_id = (SELECT id FROM location_groups WHERE created_by = exports.user_id      AND tenant_id = exports.tenant_id      LIMIT 1);
UPDATE images       SET group_id = (SELECT id FROM location_groups WHERE created_by = images.user_id       AND tenant_id = images.tenant_id       LIMIT 1);
UPDATE manuals      SET group_id = (SELECT id FROM location_groups WHERE created_by = manuals.user_id      AND tenant_id = manuals.tenant_id      LIMIT 1);
UPDATE invoices     SET group_id = (SELECT id FROM location_groups WHERE created_by = invoices.user_id     AND tenant_id = invoices.tenant_id     LIMIT 1);
UPDATE restore_operations SET group_id = (SELECT id FROM location_groups WHERE created_by = restore_operations.user_id AND tenant_id = restore_operations.tenant_id LIMIT 1);
UPDATE restore_steps      SET group_id = (SELECT id FROM location_groups WHERE created_by = restore_steps.user_id      AND tenant_id = restore_steps.tenant_id      LIMIT 1);

-- Step 4: Make group_id NOT NULL on all data tables
ALTER TABLE locations           ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE areas               ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE commodities         ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE files               ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE exports             ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE images              ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE manuals             ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE invoices            ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE restore_operations  ALTER COLUMN group_id SET NOT NULL;
ALTER TABLE restore_steps       ALTER COLUMN group_id SET NOT NULL;

-- Step 5: Rename user_id to created_by_user_id on all data tables (AFTER backfill)
ALTER TABLE locations           RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE areas               RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE commodities         RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE files               RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE exports             RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE images              RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE manuals             RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE invoices            RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE restore_operations  RENAME COLUMN user_id TO created_by_user_id;
ALTER TABLE restore_steps       RENAME COLUMN user_id TO created_by_user_id;

-- Step 6: Update RLS policies on all data tables from user_id-based to group_id-based
-- Drop old policies (tenant + user) and create new ones (tenant + group)
-- This list must match the 10 data tables that switched to TenantGroupAwareEntityID.

DO $$
DECLARE
    tbl TEXT;
    policy_name TEXT;
    new_using TEXT;
BEGIN
    new_using := 'tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '''' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''''';


    FOR tbl, policy_name IN VALUES
        ('locations',          'location_isolation'),
        ('areas',              'area_isolation'),
        ('commodities',        'commodity_isolation'),
        ('files',              'file_isolation'),
        ('exports',            'export_isolation'),
        ('images',             'image_isolation'),
        ('invoices',           'invoice_isolation'),
        ('manuals',            'manual_isolation'),
        ('restore_operations', 'restore_operation_isolation'),
        ('restore_steps',      'restore_step_isolation')
    LOOP
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', policy_name, tbl);
        EXECUTE format(
            'CREATE POLICY %I ON %I FOR ALL TO inventario_app USING (%s) WITH CHECK (%s)',
            policy_name, tbl, new_using, new_using
        );
    END LOOP;
END
$$;

-- Step 7: Remove role column from users table
DROP INDEX IF EXISTS users_role_idx;
ALTER TABLE users DROP COLUMN role;
