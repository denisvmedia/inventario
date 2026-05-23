-- Migration rollback
-- Generated on: 2026-05-17T08:12:49Z
-- Direction: DOWN

-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN sale_price CASCADE;
-- WARNING: Dropping column commodities.sale_price with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN status_date CASCADE;
-- WARNING: Dropping column commodities.status_date with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN status_note CASCADE;
-- WARNING: Dropping column commodities.status_note with CASCADE - This will delete data and dependent objects! --;