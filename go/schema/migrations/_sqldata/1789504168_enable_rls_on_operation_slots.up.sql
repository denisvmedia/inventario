-- Migration generated from schema differences
-- Generated on: 2026-09-15T22:29:28+02:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Enable RLS for operation_slots table
ALTER TABLE "operation_slots" ENABLE ROW LEVEL SECURITY;
-- Allows background workers to sweep expired operation slots across all tenants
DROP POLICY IF EXISTS "operation_slot_background_worker_access" ON "operation_slots";
CREATE POLICY "operation_slot_background_worker_access" ON "operation_slots" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures operation slots can only be accessed and modified by their tenant and user with required contexts
DROP POLICY IF EXISTS "operation_slot_isolation" ON "operation_slots";
CREATE POLICY "operation_slot_isolation" ON "operation_slots" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');