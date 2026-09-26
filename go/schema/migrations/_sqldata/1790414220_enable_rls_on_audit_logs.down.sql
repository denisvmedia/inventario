-- Migration rollback
-- Generated on: 2026-09-26T11:17:00+02:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Drop RLS policy audit_log_background_worker_access from table audit_logs
DROP POLICY IF EXISTS "audit_log_background_worker_access" ON "audit_logs";
-- Drop RLS policy audit_log_tenant_isolation from table audit_logs
DROP POLICY IF EXISTS "audit_log_tenant_isolation" ON "audit_logs";
-- Disable RLS for audit_logs table
ALTER TABLE "audit_logs" DISABLE ROW LEVEL SECURITY;