-- Migration rollback
-- Generated on: 2026-04-16T20:08:38+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_group_invites_expires_at;
DROP INDEX IF EXISTS idx_group_invites_group_id;
DROP INDEX IF EXISTS idx_group_invites_tenant_id;
DROP INDEX IF EXISTS idx_group_invites_token;
DROP INDEX IF EXISTS idx_group_invites_uuid;
DROP INDEX IF EXISTS idx_group_memberships_group_id;
DROP INDEX IF EXISTS idx_group_memberships_member_user_id;
DROP INDEX IF EXISTS idx_group_memberships_tenant_id;
DROP INDEX IF EXISTS idx_group_memberships_unique;
DROP INDEX IF EXISTS idx_group_memberships_uuid;
DROP INDEX IF EXISTS idx_location_groups_status;
DROP INDEX IF EXISTS idx_location_groups_tenant_id;
DROP INDEX IF EXISTS idx_location_groups_tenant_slug;
DROP INDEX IF EXISTS idx_location_groups_uuid;
-- Drop RLS policy group_invite_background_worker_access from table group_invites
DROP POLICY IF EXISTS group_invite_background_worker_access ON group_invites;
-- Drop RLS policy group_invite_tenant_isolation from table group_invites
DROP POLICY IF EXISTS group_invite_tenant_isolation ON group_invites;
-- Drop RLS policy group_membership_background_worker_access from table group_memberships
DROP POLICY IF EXISTS group_membership_background_worker_access ON group_memberships;
-- Drop RLS policy group_membership_tenant_isolation from table group_memberships
DROP POLICY IF EXISTS group_membership_tenant_isolation ON group_memberships;
-- Drop RLS policy location_group_background_worker_access from table location_groups
DROP POLICY IF EXISTS location_group_background_worker_access ON location_groups;
-- Drop RLS policy location_group_tenant_isolation from table location_groups
DROP POLICY IF EXISTS location_group_tenant_isolation ON location_groups;
-- NOTE: RLS policies were removed from table group_invites - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table group_memberships - verify if RLS should be disabled --
-- NOTE: RLS policies were removed from table location_groups - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS group_invites CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS group_memberships CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS location_groups CASCADE;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS get_current_group_id;
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS set_group_context;