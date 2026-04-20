-- Migration rollback
-- Generated on: 2026-04-20T20:43:24+02:00
-- Direction: DOWN

ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_user_default_group;
-- Remove columns from table: users --
-- ALTER statements: --
ALTER TABLE users DROP COLUMN default_group_id CASCADE;
-- WARNING: Dropping column users.default_group_id with CASCADE - This will delete data and dependent objects! --
