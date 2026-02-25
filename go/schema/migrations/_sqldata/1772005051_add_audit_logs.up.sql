-- Migration generated from schema differences
-- Generated on: 2026-02-25T08:37:31+01:00
-- Direction: UP

-- POSTGRES TABLE: audit_logs --
CREATE TABLE audit_logs (
  id TEXT PRIMARY KEY NOT NULL,
  timestamp TIMESTAMP NOT NULL,
  user_id TEXT,
  tenant_id TEXT,
  action TEXT NOT NULL,
  entity_type TEXT,
  entity_id TEXT,
  ip_address TEXT,
  user_agent TEXT,
  success BOOLEAN NOT NULL DEFAULT 'true',
  error_message TEXT
);
CREATE INDEX IF NOT EXISTS audit_logs_action_idx ON audit_logs (action);
CREATE INDEX IF NOT EXISTS audit_logs_entity_idx ON audit_logs (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS audit_logs_tenant_id_idx ON audit_logs (tenant_id);
CREATE INDEX IF NOT EXISTS audit_logs_timestamp_idx ON audit_logs (timestamp);
CREATE INDEX IF NOT EXISTS audit_logs_user_id_idx ON audit_logs (user_id);