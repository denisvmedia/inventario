-- Migration generated from schema differences
-- Generated on: 2026-11-27T15:53:27+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "exports" ADD COLUMN "heartbeat_at" TIMESTAMP;
ALTER TABLE "restore_operations" ADD COLUMN "heartbeat_at" TIMESTAMP;