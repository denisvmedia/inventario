-- Migration: operation_slots table
-- This migration was originally bundled with 1771692316 (refresh_tokens) but
-- operation_slots belongs to the upload-rate-limiting feature (PR #608).
-- Separated here to keep migrations scoped to a single feature.
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
-- ALTER statements: --
ALTER TABLE operation_slots ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE operation_slots ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
CREATE INDEX IF NOT EXISTS idx_operation_slots_cleanup ON operation_slots (expires_at);
CREATE INDEX IF NOT EXISTS idx_operation_slots_operation ON operation_slots (operation_name, expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_slots_unique ON operation_slots (tenant_id, user_id, operation_name, slot_id);
CREATE INDEX IF NOT EXISTS idx_operation_slots_user_operation ON operation_slots (tenant_id, user_id, operation_name, expires_at);
