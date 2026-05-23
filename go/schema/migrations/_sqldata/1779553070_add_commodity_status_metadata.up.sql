-- Migration generated from schema differences
-- Generated on: 2026-05-17T08:12:49Z
-- Direction: UP

-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN sale_price DECIMAL(15,2);
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN status_date TEXT;
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN status_note TEXT;