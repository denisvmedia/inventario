-- Migration rollback
-- Generated on: 2026-05-23T12:08:19Z
-- Direction: DOWN

-- Add/modify columns for table: backoffice_users --
-- ALTER statements: --
ALTER TABLE backoffice_users ALTER COLUMN mfa_enforced TYPE boolean;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "backoffice_users" WHERE "mfa_enforced" IS NULL LIMIT 1) THEN
        UPDATE "backoffice_users" SET "mfa_enforced" = 'false' WHERE "mfa_enforced" IS NULL;
    END IF;
END
$$;
ALTER TABLE backoffice_users ALTER COLUMN mfa_enforced SET NOT NULL;
ALTER TABLE backoffice_users ALTER COLUMN mfa_enforced SET DEFAULT 'false';
-- Modify column backoffice_users.mfa_enforced: default_expr: true -> false --
DROP INDEX IF EXISTS idx_backoffice_user_mfa_secrets_user;
DROP INDEX IF EXISTS idx_backoffice_user_mfa_secrets_uuid;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS backoffice_user_mfa_secrets CASCADE;