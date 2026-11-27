-- Migration generated from schema differences
-- Generated on: 2026-11-27T18:17:02+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "commodities" ADD COLUMN "revision" BIGINT NOT NULL DEFAULT 1;