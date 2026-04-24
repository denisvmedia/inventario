-- Migration rollback
-- Generated on: 2026-04-23T21:53:04+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_group_invites_audit_archived_at;
DROP INDEX IF EXISTS idx_group_invites_audit_original_group_id;
DROP INDEX IF EXISTS idx_group_invites_audit_tenant_id;
DROP INDEX IF EXISTS idx_group_invites_audit_tenant_invite;
DROP INDEX IF EXISTS idx_group_invites_audit_used_by;
DROP INDEX IF EXISTS idx_group_invites_audit_uuid;
-- Drop RLS policy group_invite_audit_background_worker_access from table group_invites_audit
DROP POLICY IF EXISTS group_invite_audit_background_worker_access ON group_invites_audit;
-- Drop RLS policy group_invite_audit_tenant_isolation from table group_invites_audit
DROP POLICY IF EXISTS group_invite_audit_tenant_isolation ON group_invites_audit;
-- NOTE: RLS policies were removed from table group_invites_audit - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS group_invites_audit CASCADE;