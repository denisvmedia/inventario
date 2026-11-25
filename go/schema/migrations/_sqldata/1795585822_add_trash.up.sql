-- Migration generated from schema differences
-- Generated on: 2026-11-25T06:50:22+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "deleted_at" TIMESTAMP;
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "deleted_by" TEXT;
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "deleted_from_area_id" TEXT;
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE "files" ADD COLUMN "deleted_at" TIMESTAMP;
-- ALTER statements: --
ALTER TABLE "files" ADD COLUMN "deleted_by" TEXT;
-- ALTER statements: --
ALTER TABLE "files" ADD COLUMN "deleted_with_commodity_id" TEXT;
CREATE INDEX IF NOT EXISTS "commodities_deleted_at_idx" ON "commodities" ("deleted_at") WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS "files_deleted_at_idx" ON "files" ("deleted_at") WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS "files_deleted_with_commodity_idx" ON "files" ("deleted_with_commodity_id") WHERE deleted_with_commodity_id IS NOT NULL;