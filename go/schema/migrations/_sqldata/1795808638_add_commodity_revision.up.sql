-- Migration generated from schema differences
-- Generated on: 2026-11-27T20:43:58+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "commodities" ADD COLUMN "revision" BIGINT NOT NULL DEFAULT 1;