-- Migration: add user_mfa_secrets for TOTP/MFA (#1380 / #1645)
-- Direction: DOWN

DROP TABLE IF EXISTS user_mfa_secrets;
