-- Migration rollback
-- Generated on: 2026-11-25T03:40:05+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "legacy_zero_prices" CASCADE;
-- WARNING: Dropping column commodities.legacy_zero_prices with CASCADE - This will delete data and dependent objects! --;