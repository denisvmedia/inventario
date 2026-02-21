-- Migration generated from schema differences
-- Generated on: 2026-02-21T17:45:16+01:00
-- Direction: UP

-- POSTGRES TABLE: operation_slots --
CREATE TABLE operation_slots (
  slot_id INTEGER NOT NULL,
  operation_name TEXT NOT NULL DEFAULT 'upload',
  created_at TIMESTAMP NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- POSTGRES TABLE: refresh_tokens --
CREATE TABLE refresh_tokens (
  token_hash VARCHAR(128) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL,
  last_used_at TIMESTAMP,
  ip_address VARCHAR(45),
  user_agent TEXT,
  revoked_at TIMESTAMP,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL
);
-- ALTER statements: --
ALTER TABLE operation_slots ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE operation_slots ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE refresh_tokens ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE refresh_tokens ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- Enable RLS for refresh_tokens table
ALTER TABLE refresh_tokens ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all refresh tokens for cleanup
DROP POLICY IF EXISTS refresh_token_background_worker_access ON refresh_tokens;
CREATE POLICY refresh_token_background_worker_access ON refresh_tokens FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures refresh tokens can only be accessed and modified by the owning user within their tenant
DROP POLICY IF EXISTS refresh_token_isolation ON refresh_tokens;
CREATE POLICY refresh_token_isolation ON refresh_tokens FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
CREATE INDEX IF NOT EXISTS idx_operation_slots_cleanup ON operation_slots (expires_at);
CREATE INDEX IF NOT EXISTS idx_operation_slots_operation ON operation_slots (operation_name, expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_slots_unique ON operation_slots (tenant_id, user_id, operation_name, slot_id);
CREATE INDEX IF NOT EXISTS idx_operation_slots_user_operation ON operation_slots (tenant_id, user_id, operation_name, expires_at);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens (expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id);