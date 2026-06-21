-- Migration rollback
-- Generated on: 2026-06-21T13:32:11Z
-- Direction: DOWN

-- Remove columns from table: backoffice_user_mfa_secrets --
-- ALTER statements: --
ALTER TABLE backoffice_user_mfa_secrets DROP COLUMN last_used_step CASCADE;
-- WARNING: Dropping column backoffice_user_mfa_secrets.last_used_step with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: user_mfa_secrets --
-- ALTER statements: --
ALTER TABLE user_mfa_secrets DROP COLUMN last_used_step CASCADE;
-- WARNING: Dropping column user_mfa_secrets.last_used_step with CASCADE - This will delete data and dependent objects! --;