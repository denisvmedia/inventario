-- Phase 6 rollback: reverse data backfill + column rename + role + RLS restoration

-- Reverse step 7: restore role column on users
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
CREATE INDEX IF NOT EXISTS users_role_idx ON users (role);
-- Best-effort backfill from group_memberships
UPDATE users SET role = COALESCE(
    (SELECT gm.role FROM group_memberships gm WHERE gm.member_user_id = users.id LIMIT 1),
    'user'
);

-- Reverse step 5: rename back to user_id.
-- This must happen BEFORE restoring the old RLS policies below, because those
-- policies reference user_id and would fail to create against an undefined column.
ALTER TABLE locations           RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE areas               RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE commodities         RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE files               RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE exports             RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE images              RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE manuals             RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE invoices            RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE restore_operations  RENAME COLUMN created_by_user_id TO user_id;
ALTER TABLE restore_steps       RENAME COLUMN created_by_user_id TO user_id;

-- Reverse step 6: restore old RLS policies (tenant + user instead of tenant + group)
DO $$
DECLARE
    tbl TEXT;
    policy_name TEXT;
    new_using TEXT;
BEGIN
    new_using := 'tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '''' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''''';

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

-- Reverse step 4: make group_id nullable again
ALTER TABLE locations           ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE areas               ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE commodities         ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE files               ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE exports             ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE images              ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE manuals             ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE invoices            ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE restore_operations  ALTER COLUMN group_id DROP NOT NULL;
ALTER TABLE restore_steps       ALTER COLUMN group_id DROP NOT NULL;

-- Reverse step 3: clear group_id
UPDATE locations           SET group_id = NULL;
UPDATE areas               SET group_id = NULL;
UPDATE commodities         SET group_id = NULL;
UPDATE files               SET group_id = NULL;
UPDATE exports             SET group_id = NULL;
UPDATE images              SET group_id = NULL;
UPDATE manuals             SET group_id = NULL;
UPDATE invoices            SET group_id = NULL;
UPDATE restore_operations  SET group_id = NULL;
UPDATE restore_steps       SET group_id = NULL;

-- Reverse steps 1-2: delete memberships and groups that were created by the
-- Phase 6 backfill — scoped so that any group a user has renamed or any
-- group created after the migration is preserved. Backfill rows are
-- identified by two signatures set in up.sql:
--   * name matches the "<user.name>'s Group" pattern
--   * created_at = updated_at (never modified since creation)
-- A user who renamed their default group, or any group created after the
-- migration, does not match both criteria and is kept intact.
DELETE FROM group_memberships
WHERE group_id IN (
    SELECT id FROM location_groups
    WHERE name LIKE '%''s Group' AND created_at = updated_at
);

DELETE FROM location_groups
WHERE name LIKE '%''s Group' AND created_at = updated_at;
