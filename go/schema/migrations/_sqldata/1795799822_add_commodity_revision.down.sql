-- Migration rollback
-- Generated on: 2026-11-27T18:17:02+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "commodities" DROP COLUMN "revision";