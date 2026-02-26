-- Migration rollback
-- Generated on: 2026-02-26T18:13:01+01:00
-- Direction: DOWN

DROP INDEX IF EXISTS password_resets_email_idx;
DROP INDEX IF EXISTS password_resets_token_idx;
DROP INDEX IF EXISTS password_resets_user_id_idx;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS password_resets CASCADE;