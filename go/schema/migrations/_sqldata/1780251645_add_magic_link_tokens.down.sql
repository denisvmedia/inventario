-- Migration rollback
-- Generated on: 2026-05-31T18:20:45Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_magic_link_tokens_uuid;
DROP INDEX IF EXISTS magic_link_tokens_email_idx;
DROP INDEX IF EXISTS magic_link_tokens_token_idx;
DROP INDEX IF EXISTS magic_link_tokens_user_id_idx;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS magic_link_tokens CASCADE;