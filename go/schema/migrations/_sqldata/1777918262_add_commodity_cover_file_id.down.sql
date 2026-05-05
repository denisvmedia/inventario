-- Migration rollback
-- Generated on: 2026-05-04T20:11:02+02:00
-- Direction: DOWN

ALTER TABLE commodities DROP CONSTRAINT IF EXISTS fk_commodity_cover_file;
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN cover_file_id CASCADE;
-- WARNING: Dropping column commodities.cover_file_id with CASCADE - This will delete data and dependent objects! --;