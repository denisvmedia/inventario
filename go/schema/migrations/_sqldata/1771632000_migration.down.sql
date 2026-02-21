-- Migration rollback
-- Generated on: 2026-02-21T00:00:00+00:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
DROP INDEX IF EXISTS idx_refresh_tokens_token_hash;
DROP INDEX IF EXISTS idx_refresh_tokens_expires_at;
-- Drop RLS policy refresh_token_background_worker_access from table refresh_tokens
DROP POLICY IF EXISTS refresh_token_background_worker_access ON refresh_tokens;
-- Drop RLS policy refresh_token_isolation from table refresh_tokens
DROP POLICY IF EXISTS refresh_token_isolation ON refresh_tokens;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS refresh_tokens CASCADE;
