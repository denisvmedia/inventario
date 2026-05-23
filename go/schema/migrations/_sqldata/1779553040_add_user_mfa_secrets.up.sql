-- Migration: add user_mfa_secrets for TOTP/MFA (#1380 / #1645)
-- Direction: UP

-- POSTGRES TABLE: user_mfa_secrets --
CREATE TABLE user_mfa_secrets (
  secret_encrypted TEXT NOT NULL,
  enabled_at TIMESTAMP,
  backup_codes_hashed JSONB NOT NULL DEFAULT '[]',
  last_used_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE user_mfa_secrets ADD CONSTRAINT fk_user_mfa_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE user_mfa_secrets ADD CONSTRAINT fk_user_mfa_user FOREIGN KEY (user_id) REFERENCES users(id);
-- Enable RLS for user_mfa_secrets table
ALTER TABLE user_mfa_secrets ENABLE ROW LEVEL SECURITY;
-- Allows the login flow + management endpoints to read the row before RLS context is established on the connection
DROP POLICY IF EXISTS user_mfa_background_worker_access ON user_mfa_secrets;
CREATE POLICY user_mfa_background_worker_access ON user_mfa_secrets FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures MFA secrets can only be accessed and modified by the owning user within their tenant
DROP POLICY IF EXISTS user_mfa_isolation ON user_mfa_secrets;
CREATE POLICY user_mfa_isolation ON user_mfa_secrets FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_mfa_secrets_uuid ON user_mfa_secrets (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_mfa_secrets_user ON user_mfa_secrets (tenant_id, user_id);
