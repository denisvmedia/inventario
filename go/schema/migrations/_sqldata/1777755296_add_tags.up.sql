-- Migration generated from schema differences
-- Generated on: 2026-05-02T22:54:56+02:00
-- Direction: UP

-- POSTGRES TABLE: tags --
CREATE TABLE tags (
  slug TEXT NOT NULL,
  label TEXT NOT NULL,
  color TEXT NOT NULL DEFAULT 'muted',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE tags ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE tags ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE tags ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Enable RLS for tags table
ALTER TABLE tags ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all tags for processing
DROP POLICY IF EXISTS tag_background_worker_access ON tags;
CREATE POLICY tag_background_worker_access ON tags FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures tags can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS tag_isolation ON tags;
CREATE POLICY tag_isolation ON tags FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_group_slug ON tags (group_id, slug);
CREATE INDEX IF NOT EXISTS idx_tags_tenant_group ON tags (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_tags_tenant_id ON tags (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_uuid ON tags (uuid);
CREATE INDEX IF NOT EXISTS tags_label_trgm_idx ON tags USING GIN (label gin_trgm_ops);