-- Migration generated from schema differences
-- Generated on: 2026-04-18T11:31:41+02:00
-- Direction: UP

CREATE INDEX IF NOT EXISTS idx_areas_tenant_group ON areas (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_commodities_tenant_group ON commodities (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_exports_tenant_group ON exports (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_files_tenant_group ON files (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_images_tenant_group ON images (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant_group ON invoices (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_locations_tenant_group ON locations (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_manuals_tenant_group ON manuals (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_restore_operations_tenant_group ON restore_operations (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_restore_steps_tenant_group ON restore_steps (tenant_id, group_id);