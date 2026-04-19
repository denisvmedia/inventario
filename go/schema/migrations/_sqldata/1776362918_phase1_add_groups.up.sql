-- Migration generated from schema differences
-- Generated on: 2026-04-16T20:08:38+02:00
-- Direction: UP

-- Gets the current group ID from session for RLS policies
CREATE OR REPLACE FUNCTION get_current_group_id() RETURNS TEXT AS $$
BEGIN RETURN current_setting('app.current_group_id', true); END;
$$
LANGUAGE plpgsql STABLE;
-- Sets the current group context for RLS policies
CREATE OR REPLACE FUNCTION set_group_context(group_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_group_id', group_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;
-- POSTGRES TABLE: location_groups --
CREATE TABLE location_groups (
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  icon TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_by TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: group_memberships --
CREATE TABLE group_memberships (
  group_id TEXT NOT NULL,
  member_user_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'user',
  joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- POSTGRES TABLE: group_invites --
CREATE TABLE group_invites (
  group_id TEXT NOT NULL,
  token TEXT NOT NULL,
  created_by TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_by TEXT,
  used_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE location_groups ADD CONSTRAINT fk_location_group_created_by FOREIGN KEY (created_by) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE location_groups ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE location_groups ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE group_memberships ADD CONSTRAINT fk_membership_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE group_memberships ADD CONSTRAINT fk_membership_user FOREIGN KEY (member_user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE group_memberships ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE group_memberships ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE group_invites ADD CONSTRAINT fk_invite_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE group_invites ADD CONSTRAINT fk_invite_created_by FOREIGN KEY (created_by) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE group_invites ADD CONSTRAINT fk_invite_used_by FOREIGN KEY (used_by) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE group_invites ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE group_invites ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- Enable RLS for group_memberships table
ALTER TABLE group_memberships ENABLE ROW LEVEL SECURITY;
-- Enable RLS for group_invites table
ALTER TABLE group_invites ENABLE ROW LEVEL SECURITY;
-- Enable RLS for location_groups table
ALTER TABLE location_groups ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all group invites for cleanup
DROP POLICY IF EXISTS group_invite_background_worker_access ON group_invites;
CREATE POLICY group_invite_background_worker_access ON group_invites FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures group invites are isolated by tenant
DROP POLICY IF EXISTS group_invite_tenant_isolation ON group_invites;
CREATE POLICY group_invite_tenant_isolation ON group_invites FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
-- Allows background workers to access all group memberships for processing
DROP POLICY IF EXISTS group_membership_background_worker_access ON group_memberships;
CREATE POLICY group_membership_background_worker_access ON group_memberships FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures group memberships are isolated by tenant; user-level filtering happens in application logic
DROP POLICY IF EXISTS group_membership_tenant_isolation ON group_memberships;
CREATE POLICY group_membership_tenant_isolation ON group_memberships FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
-- Allows background workers to access all location groups for processing
DROP POLICY IF EXISTS location_group_background_worker_access ON location_groups;
CREATE POLICY location_group_background_worker_access ON location_groups FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures location groups are isolated by tenant; group-level access is enforced in application logic via memberships
DROP POLICY IF EXISTS location_group_tenant_isolation ON location_groups;
CREATE POLICY location_group_tenant_isolation ON location_groups FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
CREATE INDEX IF NOT EXISTS idx_group_invites_expires_at ON group_invites (expires_at);
CREATE INDEX IF NOT EXISTS idx_group_invites_group_id ON group_invites (group_id);
CREATE INDEX IF NOT EXISTS idx_group_invites_tenant_id ON group_invites (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_invites_token ON group_invites (token);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_invites_uuid ON group_invites (uuid);
CREATE INDEX IF NOT EXISTS idx_group_memberships_group_id ON group_memberships (group_id);
CREATE INDEX IF NOT EXISTS idx_group_memberships_member_user_id ON group_memberships (member_user_id);
CREATE INDEX IF NOT EXISTS idx_group_memberships_tenant_id ON group_memberships (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_memberships_unique ON group_memberships (tenant_id, group_id, member_user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_memberships_uuid ON group_memberships (uuid);
CREATE INDEX IF NOT EXISTS idx_location_groups_status ON location_groups (status);
CREATE INDEX IF NOT EXISTS idx_location_groups_tenant_id ON location_groups (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_location_groups_tenant_slug ON location_groups (tenant_id, slug);
CREATE UNIQUE INDEX IF NOT EXISTS idx_location_groups_uuid ON location_groups (uuid);