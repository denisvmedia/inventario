-- Migration generated from schema differences
-- Generated on: 2026-05-23T09:49:13Z
-- Direction: UP

-- POSTGRES TABLE: commodity_scan_audits --
CREATE TABLE commodity_scan_audits (
  provider VARCHAR(32) NOT NULL,
  model VARCHAR(64) NOT NULL,
  photo_count SMALLINT NOT NULL,
  total_photo_bytes INTEGER NOT NULL,
  status VARCHAR(16) NOT NULL,
  error_code VARCHAR(64),
  latency_ms INTEGER NOT NULL,
  tokens_used INTEGER NOT NULL DEFAULT '0',
  result_json JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE commodity_scan_audits ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE commodity_scan_audits ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- Enable RLS for commodity_scan_audits table
ALTER TABLE commodity_scan_audits ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all commodity scan audit rows for retention/analytics
DROP POLICY IF EXISTS commodity_scan_audit_background_worker_access ON commodity_scan_audits;
CREATE POLICY commodity_scan_audit_background_worker_access ON commodity_scan_audits FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures commodity scan audit rows can only be accessed and modified by the owning user within their tenant
DROP POLICY IF EXISTS commodity_scan_audit_isolation ON commodity_scan_audits;
CREATE POLICY commodity_scan_audit_isolation ON commodity_scan_audits FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
CREATE INDEX IF NOT EXISTS idx_commodity_scan_audits_tenant_created ON commodity_scan_audits (tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_commodity_scan_audits_user_created ON commodity_scan_audits (user_id, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_commodity_scan_audits_uuid ON commodity_scan_audits (uuid);