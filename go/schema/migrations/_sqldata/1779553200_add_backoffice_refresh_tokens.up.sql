-- Migration: add backoffice_refresh_tokens for the back-office auth plane (#1785)
-- Direction: UP
--
-- backoffice_refresh_tokens stores long-lived refresh tokens for the
-- back-office auth plane (issue #1785, Phase 2). Mirrors refresh_tokens
-- in shape but is FK'd to backoffice_users rather than (tenant_id,
-- user_id) so the two identity universes can't share a row. The table
-- has NO row-level security — same reason as backoffice_users: it lives
-- OUTSIDE the tenant model, and the login flow runs before any DB
-- session context is set, so an RLS predicate that read
-- `get_current_*_id()` would block the very call that needs to
-- authenticate. Access is gated entirely at the application layer.

-- POSTGRES TABLE: backoffice_refresh_tokens --
CREATE TABLE backoffice_refresh_tokens (
  backoffice_user_id TEXT NOT NULL,
  token_hash VARCHAR(128) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_used_at TIMESTAMP,
  ip_address VARCHAR(45),
  user_agent TEXT,
  revoked_at TIMESTAMP,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE backoffice_refresh_tokens ADD CONSTRAINT fk_backoffice_refresh_token_user FOREIGN KEY (backoffice_user_id) REFERENCES backoffice_users(id);
CREATE INDEX IF NOT EXISTS idx_backoffice_refresh_tokens_expires_at ON backoffice_refresh_tokens (expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_backoffice_refresh_tokens_token_hash ON backoffice_refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_backoffice_refresh_tokens_user_id ON backoffice_refresh_tokens (backoffice_user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_backoffice_refresh_tokens_uuid ON backoffice_refresh_tokens (uuid);
