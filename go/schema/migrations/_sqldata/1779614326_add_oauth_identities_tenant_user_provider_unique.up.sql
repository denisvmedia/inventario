-- Migration generated from schema differences
-- Generated on: 2026-05-24T11:18:46+02:00
-- Direction: UP

CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_identities_tenant_user_provider ON user_oauth_identities (tenant_id, user_id, provider);