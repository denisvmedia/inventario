-- Migration generated from schema differences
-- Generated on: 2026-02-26T18:13:01+01:00
-- Direction: UP

-- POSTGRES TABLE: password_resets --
CREATE TABLE password_resets (
  id TEXT PRIMARY KEY NOT NULL,
  user_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  email TEXT NOT NULL,
  token TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  used_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL
);
-- ALTER statements: --
ALTER TABLE password_resets ADD CONSTRAINT fk_password_reset_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE password_resets ADD CONSTRAINT fk_password_reset_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
CREATE INDEX IF NOT EXISTS password_resets_email_idx ON password_resets (email);
CREATE UNIQUE INDEX IF NOT EXISTS password_resets_token_idx ON password_resets (token);
CREATE INDEX IF NOT EXISTS password_resets_user_id_idx ON password_resets (user_id);