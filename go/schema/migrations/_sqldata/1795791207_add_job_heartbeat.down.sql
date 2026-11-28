-- Migration rollback
-- Generated on: 2026-11-27T15:53:27+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "exports" DROP COLUMN "heartbeat_at";
ALTER TABLE "restore_operations" DROP COLUMN "heartbeat_at";