-- Migration generated from schema differences
-- Generated on: 2026-05-23T08:32:34Z
-- Direction: UP

-- POSTGRES TABLE: system_admin_grants --
CREATE TABLE system_admin_grants (
  user_id TEXT NOT NULL,
  granted_by TEXT,
  granted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE system_admin_grants ADD CONSTRAINT fk_system_admin_grants_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE system_admin_grants ADD CONSTRAINT fk_system_admin_grants_granted_by FOREIGN KEY (granted_by) REFERENCES users(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_system_admin_grants_uuid ON system_admin_grants (uuid);
CREATE UNIQUE INDEX IF NOT EXISTS system_admin_grants_user_id_idx ON system_admin_grants (user_id);