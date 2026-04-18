-- Migration rollback
-- Generated on: 2026-04-18T11:31:41+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_areas_tenant_group;
DROP INDEX IF EXISTS idx_commodities_tenant_group;
DROP INDEX IF EXISTS idx_exports_tenant_group;
DROP INDEX IF EXISTS idx_files_tenant_group;
DROP INDEX IF EXISTS idx_images_tenant_group;
DROP INDEX IF EXISTS idx_invoices_tenant_group;
DROP INDEX IF EXISTS idx_locations_tenant_group;
DROP INDEX IF EXISTS idx_manuals_tenant_group;
DROP INDEX IF EXISTS idx_restore_operations_tenant_group;
DROP INDEX IF EXISTS idx_restore_steps_tenant_group;