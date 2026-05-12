-- Migration: 4-role taxonomy + email-based invites (issue #1533)
-- Direction: UP
--
-- Two related schema changes ship together:
--
-- 1. group_invites gains two columns:
--    - invitee_email: the address the email-invite flow targets. Stays
--      NULL for the legacy token-only path (admin generates a URL and
--      copy-pastes it) so existing invites keep validating.
--    - role: the role the invitee will be granted on acceptance.
--      Default 'user' matches the previous behaviour where every
--      AcceptInvite call promoted to user-tier.
--    A partial index on invitee_email lets resend / dedupe lookups skip
--    the legacy rows (which dominate at the time of this migration).
--
-- 2. The role enum grows from {admin, user} to {viewer, user, admin,
--    owner}. The new roles are additive — viewer is a strictly weaker
--    member than user, and owner is strictly stronger than admin. To
--    keep every existing group with a user who can delete it, every
--    current admin membership is promoted to owner. Admins who want to
--    demote co-owners can do so afterwards through the UI.

ALTER TABLE group_invites ADD COLUMN invitee_email TEXT;
ALTER TABLE group_invites ADD COLUMN role TEXT NOT NULL DEFAULT 'user';

CREATE INDEX IF NOT EXISTS idx_group_invites_invitee_email
    ON group_invites (invitee_email)
    WHERE invitee_email IS NOT NULL;

UPDATE group_memberships SET role = 'owner' WHERE role = 'admin';
