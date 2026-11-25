-- Migration generated from schema differences
-- Generated on: 2026-11-25T07:21:43+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "brand" TEXT NOT NULL DEFAULT '';
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "model" TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS "commodities_brand_trgm_idx" ON "commodities" USING GIN ("brand" gin_trgm_ops);
CREATE INDEX IF NOT EXISTS "commodities_model_trgm_idx" ON "commodities" USING GIN ("model" gin_trgm_ops);