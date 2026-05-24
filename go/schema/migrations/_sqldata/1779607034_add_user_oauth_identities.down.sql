-- Migration rollback
-- Generated on: 2026-05-24T09:17:14+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_oauth_identities_provider_subject;
DROP INDEX IF EXISTS idx_oauth_identities_tenant_id;
DROP INDEX IF EXISTS idx_oauth_identities_user_id;
DROP INDEX IF EXISTS idx_oauth_identities_uuid;
-- Drop RLS policy oauth_identity_background_worker_access from table user_oauth_identities
DROP POLICY IF EXISTS oauth_identity_background_worker_access ON user_oauth_identities;
-- Drop RLS policy oauth_identity_user_isolation from table user_oauth_identities
DROP POLICY IF EXISTS oauth_identity_user_isolation ON user_oauth_identities;
-- NOTE: RLS policies were removed from table user_oauth_identities - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS user_oauth_identities CASCADE;