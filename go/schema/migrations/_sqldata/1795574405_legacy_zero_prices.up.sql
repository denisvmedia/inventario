-- Migration generated from schema differences
-- Generated on: 2026-11-25T03:40:05+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "legacy_zero_prices" BOOLEAN NOT NULL DEFAULT true;