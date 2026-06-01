-- Migration generated from schema differences
-- Generated on: 2026-05-31T19:25:09Z
-- Direction: UP

-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ALTER COLUMN area_id TYPE TEXT;
ALTER TABLE commodities ALTER COLUMN area_id DROP NOT NULL;
ALTER TABLE commodities ALTER COLUMN area_id DROP DEFAULT;
-- Modify column commodities.area_id: nullable: false -> true --;