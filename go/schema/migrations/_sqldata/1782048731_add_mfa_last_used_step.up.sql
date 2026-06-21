-- Migration generated from schema differences
-- Generated on: 2026-06-21T13:32:11Z
-- Direction: UP

-- Add/modify columns for table: backoffice_user_mfa_secrets --
-- ALTER statements: --
ALTER TABLE backoffice_user_mfa_secrets ADD COLUMN last_used_step BIGINT NOT NULL DEFAULT '0';
-- Add/modify columns for table: user_mfa_secrets --
-- ALTER statements: --
ALTER TABLE user_mfa_secrets ADD COLUMN last_used_step BIGINT NOT NULL DEFAULT '0';