-- Migration rollback
-- Generated on: 2026-02-21T17:45:16+01:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_refresh_tokens_expires_at;
DROP INDEX IF EXISTS idx_refresh_tokens_token_hash;
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
-- Drop RLS policy refresh_token_background_worker_access from table refresh_tokens
DROP POLICY IF EXISTS refresh_token_background_worker_access ON refresh_tokens;
-- Drop RLS policy refresh_token_isolation from table refresh_tokens
DROP POLICY IF EXISTS refresh_token_isolation ON refresh_tokens;
-- NOTE: RLS policies were removed from table refresh_tokens - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS refresh_tokens CASCADE;
