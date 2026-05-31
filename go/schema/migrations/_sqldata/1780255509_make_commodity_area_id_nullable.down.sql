-- Migration rollback
-- Generated on: 2026-05-31T19:25:09Z
-- Direction: DOWN

-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ALTER COLUMN area_id TYPE text;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM "commodities" WHERE "area_id" IS NULL LIMIT 1) THEN
        UPDATE "commodities" SET "area_id" = '' WHERE "area_id" IS NULL;
    END IF;
END
$$;
ALTER TABLE commodities ALTER COLUMN area_id SET NOT NULL;
ALTER TABLE commodities ALTER COLUMN area_id DROP DEFAULT;
-- Modify column commodities.area_id: nullable: true -> false --;