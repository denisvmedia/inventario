-- Migration rollback
-- Generated on: 2026-02-26T12:30:50+01:00
-- Direction: DOWN

DROP INDEX IF EXISTS email_verifications_email_idx;
DROP INDEX IF EXISTS email_verifications_token_idx;
DROP INDEX IF EXISTS email_verifications_user_id_idx;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS email_verifications CASCADE;