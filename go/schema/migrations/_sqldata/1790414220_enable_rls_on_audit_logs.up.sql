-- Migration generated from schema differences
-- Generated on: 2026-09-26T11:17:00+02:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Enable RLS for audit_logs table
ALTER TABLE "audit_logs" ENABLE ROW LEVEL SECURITY;
-- Allows the back-office plane and workers to read and write audit rows across tenants
DROP POLICY IF EXISTS "audit_log_background_worker_access" ON "audit_logs";
CREATE POLICY "audit_log_background_worker_access" ON "audit_logs" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Audit rows are tenant-isolated; a system event with no tenant belongs to none
DROP POLICY IF EXISTS "audit_log_tenant_isolation" ON "audit_logs";
CREATE POLICY "audit_log_tenant_isolation" ON "audit_logs" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');