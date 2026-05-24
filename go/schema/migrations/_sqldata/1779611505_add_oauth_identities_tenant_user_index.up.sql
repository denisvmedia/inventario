-- Migration generated from schema differences
-- Generated on: 2026-05-24T10:31:45+02:00
-- Direction: UP

CREATE INDEX IF NOT EXISTS idx_oauth_identities_tenant_user ON user_oauth_identities (tenant_id, user_id);
DROP INDEX IF EXISTS idx_oauth_identities_user_id;