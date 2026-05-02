-- Recreate the legacy commodity-scoped attachment tables as empty.
--
-- The down migration is provided so operators can roll the schema back if
-- this migration trips a guard, but the rows that lived in these tables are
-- NOT restored — their data lives in `files` (see the up-migration header
-- and #1421 for the rationale). The schema below mirrors the post-Phase-8
-- shape: TenantGroupAware columns + the FileEntity-style file metadata
-- columns, NOT the original `user_id`-only shape from migration 1757874749.

CREATE TABLE IF NOT EXISTS images (
    id                 TEXT PRIMARY KEY NOT NULL,
    uuid               TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
    tenant_id          TEXT NOT NULL,
    group_id           TEXT NOT NULL,
    created_by_user_id TEXT NOT NULL,
    commodity_id       TEXT NOT NULL,
    path               TEXT NOT NULL,
    original_path      TEXT NOT NULL,
    ext                TEXT NOT NULL,
    mime_type          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS invoices (
    id                 TEXT PRIMARY KEY NOT NULL,
    uuid               TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
    tenant_id          TEXT NOT NULL,
    group_id           TEXT NOT NULL,
    created_by_user_id TEXT NOT NULL,
    commodity_id       TEXT NOT NULL,
    path               TEXT NOT NULL,
    original_path      TEXT NOT NULL,
    ext                TEXT NOT NULL,
    mime_type          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS manuals (
    id                 TEXT PRIMARY KEY NOT NULL,
    uuid               TEXT NOT NULL DEFAULT (gen_random_uuid())::text,
    tenant_id          TEXT NOT NULL,
    group_id           TEXT NOT NULL,
    created_by_user_id TEXT NOT NULL,
    commodity_id       TEXT NOT NULL,
    path               TEXT NOT NULL,
    original_path      TEXT NOT NULL,
    ext                TEXT NOT NULL,
    mime_type          TEXT NOT NULL
);

-- RLS — tenant + group isolation, matching the policies the Phase-7+
-- migrations ended up with on the live tables.
ALTER TABLE images   ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE manuals  ENABLE ROW LEVEL SECURITY;

CREATE POLICY image_isolation ON images FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE POLICY image_background_worker_access ON images FOR ALL TO inventario_background_worker
    USING (true) WITH CHECK (true);

CREATE POLICY invoice_isolation ON invoices FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE POLICY invoice_background_worker_access ON invoices FOR ALL TO inventario_background_worker
    USING (true) WITH CHECK (true);

CREATE POLICY manual_isolation ON manuals FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE POLICY manual_background_worker_access ON manuals FOR ALL TO inventario_background_worker
    USING (true) WITH CHECK (true);
