-- Migration rollback
-- Generated on: 2026-04-18T15:45:29+02:00
-- Direction: DOWN

-- Add/modify columns for table: group_invites --
-- ALTER statements: --
ALTER TABLE group_invites ADD COLUMN user_id text NOT NULL;
-- Add/modify columns for table: group_memberships --
-- ALTER statements: --
ALTER TABLE group_memberships ADD COLUMN user_id text NOT NULL;
-- Add/modify columns for table: location_groups --
-- ALTER statements: --
ALTER TABLE location_groups ADD COLUMN user_id text NOT NULL;