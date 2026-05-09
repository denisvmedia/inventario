-- Migration: rename FileCategory enum value `photos` → `images`.
-- Direction: DOWN
--
-- Inverse of the UP migration: rewrite every `images` row back to
-- `photos`. Idempotent. Any rows that were inserted with category =
-- 'images' AFTER the UP migration ran (i.e. the new code persisting
-- new rows) will also be rewritten — that is the intended semantic of
-- a rollback for this rename.

UPDATE files SET category = 'photos', updated_at = CURRENT_TIMESTAMP WHERE category = 'images';
