-- Migration generated from schema differences
-- Generated on: 2026-09-26T15:51:58+02:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Add/modify columns for table: audit_logs --
-- ALTER statements: --
ALTER TABLE "audit_logs" ADD COLUMN "request_id" TEXT;
CREATE INDEX IF NOT EXISTS "audit_logs_request_id_idx" ON "audit_logs" ("request_id");