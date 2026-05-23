-- Migration: add backoffice_users for the back-office auth plane (#1785)
-- Direction: UP
--
-- backoffice_users stores platform-operator identities used by the
-- back-office auth plane (issue #1785). The table has NO row-level
-- security — same reason as `tenants`: it IS the boundary. There is no
-- tenant_id column because back-office identities are cross-tenant by
-- design. Email is unique platform-wide; the registry layer lowercases
-- emails on read + write so case variants collapse to a single row.

-- POSTGRES TABLE: backoffice_users --
CREATE TABLE backoffice_users (
  email TEXT NOT NULL,
  name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT 'true',
  mfa_enforced BOOLEAN NOT NULL DEFAULT 'true',
  last_login_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
CREATE INDEX IF NOT EXISTS idx_backoffice_users_active ON backoffice_users (is_active);
CREATE UNIQUE INDEX IF NOT EXISTS idx_backoffice_users_email ON backoffice_users (email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_backoffice_users_uuid ON backoffice_users (uuid);