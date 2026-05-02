-- Migration rollback
-- Generated on: 2026-05-02T17:07:51+02:00
-- Direction: DOWN

-- POSTGRES TABLE: images --
CREATE TABLE images (
  commodity_id text NOT NULL,
  tenant_id text NOT NULL,
  created_by_user_id text NOT NULL,
  id text PRIMARY KEY NOT NULL,
  path text NOT NULL,
  original_path text NOT NULL,
  ext text NOT NULL,
  mime_type text NOT NULL,
  uuid text NOT NULL DEFAULT '(gen_random_uuid())::text',
  group_id text NOT NULL
);
-- POSTGRES TABLE: invoices --
CREATE TABLE invoices (
  commodity_id text NOT NULL,
  tenant_id text NOT NULL,
  created_by_user_id text NOT NULL,
  id text PRIMARY KEY NOT NULL,
  path text NOT NULL,
  original_path text NOT NULL,
  ext text NOT NULL,
  mime_type text NOT NULL,
  uuid text NOT NULL DEFAULT '(gen_random_uuid())::text',
  group_id text NOT NULL
);
-- POSTGRES TABLE: manuals --
CREATE TABLE manuals (
  commodity_id text NOT NULL,
  tenant_id text NOT NULL,
  created_by_user_id text NOT NULL,
  id text PRIMARY KEY NOT NULL,
  path text NOT NULL,
  original_path text NOT NULL,
  ext text NOT NULL,
  mime_type text NOT NULL,
  uuid text NOT NULL DEFAULT '(gen_random_uuid())::text',
  group_id text NOT NULL
);
-- Enable RLS for manuals table
ALTER TABLE manuals ENABLE ROW LEVEL SECURITY;
-- Enable RLS for invoices table
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
-- Enable RLS for images table
ALTER TABLE images ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS image_background_worker_access ON images;
CREATE POLICY image_background_worker_access ON images FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
DROP POLICY IF EXISTS image_isolation ON images;
CREATE POLICY image_isolation ON images FOR ALL TO inventario_app
    USING (((tenant_id = get_current_tenant_id()) AND (get_current_tenant_id() IS NOT NULL) AND (get_current_tenant_id() <> ''::text) AND (group_id = get_current_group_id()) AND (get_current_group_id() IS NOT NULL) AND (get_current_group_id() <> ''::text)))
    WITH CHECK (((tenant_id = get_current_tenant_id()) AND (get_current_tenant_id() IS NOT NULL) AND (get_current_tenant_id() <> ''::text) AND (group_id = get_current_group_id()) AND (get_current_group_id() IS NOT NULL) AND (get_current_group_id() <> ''::text)));
DROP POLICY IF EXISTS invoice_background_worker_access ON invoices;
CREATE POLICY invoice_background_worker_access ON invoices FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
DROP POLICY IF EXISTS invoice_isolation ON invoices;
CREATE POLICY invoice_isolation ON invoices FOR ALL TO inventario_app
    USING (((tenant_id = get_current_tenant_id()) AND (get_current_tenant_id() IS NOT NULL) AND (get_current_tenant_id() <> ''::text) AND (group_id = get_current_group_id()) AND (get_current_group_id() IS NOT NULL) AND (get_current_group_id() <> ''::text)))
    WITH CHECK (((tenant_id = get_current_tenant_id()) AND (get_current_tenant_id() IS NOT NULL) AND (get_current_tenant_id() <> ''::text) AND (group_id = get_current_group_id()) AND (get_current_group_id() IS NOT NULL) AND (get_current_group_id() <> ''::text)));
DROP POLICY IF EXISTS manual_background_worker_access ON manuals;
CREATE POLICY manual_background_worker_access ON manuals FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
DROP POLICY IF EXISTS manual_isolation ON manuals;
CREATE POLICY manual_isolation ON manuals FOR ALL TO inventario_app
    USING (((tenant_id = get_current_tenant_id()) AND (get_current_tenant_id() IS NOT NULL) AND (get_current_tenant_id() <> ''::text) AND (group_id = get_current_group_id()) AND (get_current_group_id() IS NOT NULL) AND (get_current_group_id() <> ''::text)))
    WITH CHECK (((tenant_id = get_current_tenant_id()) AND (get_current_tenant_id() IS NOT NULL) AND (get_current_tenant_id() <> ''::text) AND (group_id = get_current_group_id()) AND (get_current_group_id() IS NOT NULL) AND (get_current_group_id() <> ''::text)));
CREATE INDEX IF NOT EXISTS idx_images_tenant_commodity ON images (tenant_id, commodity_id);
CREATE INDEX IF NOT EXISTS idx_images_tenant_group ON images (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_images_tenant_id ON images (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_images_uuid ON images (uuid);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant_commodity ON invoices (tenant_id, commodity_id);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant_group ON invoices (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant_id ON invoices (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_uuid ON invoices (uuid);
CREATE INDEX IF NOT EXISTS idx_manuals_tenant_commodity ON manuals (tenant_id, commodity_id);
CREATE INDEX IF NOT EXISTS idx_manuals_tenant_group ON manuals (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_manuals_tenant_id ON manuals (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_manuals_uuid ON manuals (uuid);