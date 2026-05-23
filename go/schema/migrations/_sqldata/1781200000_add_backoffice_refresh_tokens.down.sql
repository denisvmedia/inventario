-- Migration rollback: drop backoffice_refresh_tokens (#1785)
-- Direction: DOWN

DROP INDEX IF EXISTS idx_backoffice_refresh_tokens_expires_at;
DROP INDEX IF EXISTS idx_backoffice_refresh_tokens_token_hash;
DROP INDEX IF EXISTS idx_backoffice_refresh_tokens_user_id;
DROP INDEX IF EXISTS idx_backoffice_refresh_tokens_uuid;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS backoffice_refresh_tokens CASCADE;
