-- Migration rollback
-- Generated on: 2026-09-26T15:51:58+02:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "audit_logs_request_id_idx";
-- Remove columns from table: audit_logs --
-- ALTER statements: --
ALTER TABLE "audit_logs" DROP COLUMN "request_id" CASCADE;
-- WARNING: Dropping column audit_logs.request_id with CASCADE - This will delete data and dependent objects! --;