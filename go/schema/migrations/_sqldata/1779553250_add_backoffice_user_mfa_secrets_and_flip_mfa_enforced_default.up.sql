-- Migration generated from schema differences
-- Generated on: 2026-05-23T12:08:19Z
-- Direction: UP

-- POSTGRES TABLE: backoffice_user_mfa_secrets --
CREATE TABLE backoffice_user_mfa_secrets (
  backoffice_user_id TEXT NOT NULL,
  secret_encrypted TEXT NOT NULL,
  enabled_at TIMESTAMP,
  backup_codes_hashed JSONB NOT NULL DEFAULT '[]',
  last_used_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE backoffice_user_mfa_secrets ADD CONSTRAINT fk_backoffice_mfa_user FOREIGN KEY (backoffice_user_id) REFERENCES backoffice_users(id);
-- Add/modify columns for table: backoffice_users --
-- ALTER statements: --
ALTER TABLE backoffice_users ALTER COLUMN mfa_enforced TYPE BOOLEAN;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "backoffice_users" WHERE "mfa_enforced" IS NULL LIMIT 1) THEN
        UPDATE "backoffice_users" SET "mfa_enforced" = 'true' WHERE "mfa_enforced" IS NULL;
    END IF;
END
$$;
ALTER TABLE backoffice_users ALTER COLUMN mfa_enforced SET NOT NULL;
ALTER TABLE backoffice_users ALTER COLUMN mfa_enforced SET DEFAULT 'true';
-- Modify column backoffice_users.mfa_enforced: default_expr: false -> true --
CREATE UNIQUE INDEX IF NOT EXISTS idx_backoffice_user_mfa_secrets_user ON backoffice_user_mfa_secrets (backoffice_user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_backoffice_user_mfa_secrets_uuid ON backoffice_user_mfa_secrets (uuid);