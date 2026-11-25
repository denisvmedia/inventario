-- Migration rollback
-- Generated on: 2026-11-25T06:50:22+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "commodities_deleted_at_idx";
DROP INDEX IF EXISTS "files_deleted_at_idx";
DROP INDEX IF EXISTS "files_deleted_with_commodity_idx";
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "deleted_at" CASCADE;
-- WARNING: Dropping column commodities.deleted_at with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "deleted_by" CASCADE;
-- WARNING: Dropping column commodities.deleted_by with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "deleted_from_area_id" CASCADE;
-- WARNING: Dropping column commodities.deleted_from_area_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE "files" DROP COLUMN "deleted_at" CASCADE;
-- WARNING: Dropping column files.deleted_at with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE "files" DROP COLUMN "deleted_by" CASCADE;
-- WARNING: Dropping column files.deleted_by with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE "files" DROP COLUMN "deleted_with_commodity_id" CASCADE;
-- WARNING: Dropping column files.deleted_with_commodity_id with CASCADE - This will delete data and dependent objects! --;