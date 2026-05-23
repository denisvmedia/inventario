-- Migration: 4-role taxonomy + email-based invites (issue #1533)
-- Direction: DOWN
--
-- Reverts the schema additions and demotes owners back to admin. The
-- demotion is safe because `owner` did not exist before the UP, so
-- every `owner` row in the table today must have been minted by it (or
-- by user action AFTER the migration ran). Demoting them back to admin
-- restores the pre-#1533 invariant of admin-only-and-user.
--
-- Viewer memberships, if any have been created post-UP, are NOT
-- demoted — they remain as `viewer` strings in a TEXT column. If we
-- reverted the application binary alongside this DOWN, GroupRole
-- validation would reject those rows on read. That's acceptable: the
-- DOWN is a recovery path, not a routine operation, and an operator
-- running it should manually `UPDATE ... SET role = 'user' WHERE role
-- = 'viewer'` if they need a clean revert.

UPDATE group_memberships SET role = 'admin' WHERE role = 'owner';

DROP INDEX IF EXISTS idx_group_invites_invitee_email;
ALTER TABLE group_invites DROP COLUMN IF EXISTS role;
ALTER TABLE group_invites DROP COLUMN IF EXISTS invitee_email;
