-- Migration generated from schema differences
-- Generated on: 2026-04-23T21:53:04+02:00
-- Direction: UP

-- POSTGRES TABLE: group_invites_audit --
CREATE TABLE group_invites_audit (
  original_invite_id TEXT NOT NULL,
  original_invite_uuid TEXT NOT NULL,
  original_group_id TEXT NOT NULL,
  original_group_slug TEXT NOT NULL,
  original_group_name TEXT NOT NULL,
  token TEXT NOT NULL,
  created_by TEXT NOT NULL,
  used_by TEXT NOT NULL,
  original_created_at TIMESTAMP NOT NULL,
  original_expires_at TIMESTAMP NOT NULL,
  used_at TIMESTAMP NOT NULL,
  archived_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE group_invites_audit ADD CONSTRAINT fk_invite_audit_created_by FOREIGN KEY (created_by) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE group_invites_audit ADD CONSTRAINT fk_invite_audit_used_by FOREIGN KEY (used_by) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE group_invites_audit ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- Enable RLS for group_invites_audit table
ALTER TABLE group_invites_audit ENABLE ROW LEVEL SECURITY;
-- Allows background workers to insert audit rows during group purge
DROP POLICY IF EXISTS group_invite_audit_background_worker_access ON group_invites_audit;
CREATE POLICY group_invite_audit_background_worker_access ON group_invites_audit FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures group invite audit records are isolated by tenant
DROP POLICY IF EXISTS group_invite_audit_tenant_isolation ON group_invites_audit;
CREATE POLICY group_invite_audit_tenant_isolation ON group_invites_audit FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
CREATE INDEX IF NOT EXISTS idx_group_invites_audit_archived_at ON group_invites_audit (archived_at);
CREATE INDEX IF NOT EXISTS idx_group_invites_audit_original_group_id ON group_invites_audit (original_group_id);
CREATE INDEX IF NOT EXISTS idx_group_invites_audit_tenant_id ON group_invites_audit (tenant_id);
CREATE INDEX IF NOT EXISTS idx_group_invites_audit_used_by ON group_invites_audit (used_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_invites_audit_uuid ON group_invites_audit (uuid);