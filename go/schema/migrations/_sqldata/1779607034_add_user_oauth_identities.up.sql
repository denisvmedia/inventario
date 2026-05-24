-- Migration generated from schema differences
-- Generated on: 2026-05-24T09:17:14+02:00
-- Direction: UP

-- POSTGRES TABLE: user_oauth_identities --
CREATE TABLE user_oauth_identities (
  user_id TEXT NOT NULL,
  provider TEXT NOT NULL,
  provider_user_id TEXT NOT NULL,
  email TEXT NOT NULL,
  linked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE user_oauth_identities ADD CONSTRAINT fk_oauth_identity_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE user_oauth_identities ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- Enable RLS for user_oauth_identities table
ALTER TABLE user_oauth_identities ENABLE ROW LEVEL SECURITY;
-- OAuth callback runs before any user session exists; uses background-worker role to look up identities by (provider, provider_user_id)
DROP POLICY IF EXISTS oauth_identity_background_worker_access ON user_oauth_identities;
CREATE POLICY oauth_identity_background_worker_access ON user_oauth_identities FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Users can read and modify only their own OAuth identities
DROP POLICY IF EXISTS oauth_identity_user_isolation ON user_oauth_identities;
CREATE POLICY oauth_identity_user_isolation ON user_oauth_identities FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_identities_provider_subject ON user_oauth_identities (provider, provider_user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_identities_tenant_id ON user_oauth_identities (tenant_id);
CREATE INDEX IF NOT EXISTS idx_oauth_identities_user_id ON user_oauth_identities (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_identities_uuid ON user_oauth_identities (uuid);