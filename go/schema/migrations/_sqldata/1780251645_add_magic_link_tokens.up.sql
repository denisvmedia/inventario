-- Migration generated from schema differences
-- Generated on: 2026-05-31T18:20:45Z
-- Direction: UP

-- POSTGRES TABLE: magic_link_tokens --
CREATE TABLE magic_link_tokens (
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
  user_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  email TEXT NOT NULL,
  token TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  claimed_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- ALTER statements: --
ALTER TABLE magic_link_tokens ADD CONSTRAINT fk_magic_link_token_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE magic_link_tokens ADD CONSTRAINT fk_magic_link_token_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_magic_link_tokens_uuid ON magic_link_tokens (uuid);
CREATE INDEX IF NOT EXISTS magic_link_tokens_email_idx ON magic_link_tokens (email);
CREATE UNIQUE INDEX IF NOT EXISTS magic_link_tokens_token_idx ON magic_link_tokens (token);
CREATE INDEX IF NOT EXISTS magic_link_tokens_user_id_idx ON magic_link_tokens (user_id);