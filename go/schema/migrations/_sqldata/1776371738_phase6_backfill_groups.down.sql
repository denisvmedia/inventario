-- Phase 6 rollback: reverse data backfill + column rename + role restoration

-- Reverse step 6: restore role column on users
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
CREATE INDEX IF NOT EXISTS users_role_idx ON users (role);
-- Best-effort backfill from group_memberships
UPDATE users SET role = COALESCE(
    (SELECT gm.role FROM group_memberships gm WHERE gm.member_user_id = users.id LIMIT 1),
    'user'
);

-- Reverse step 5: rename back to user_id
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

-- Reverse steps 1-2: delete all memberships and groups created by backfill
DELETE FROM group_memberships;
DELETE FROM location_groups;
