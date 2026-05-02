-- Migration rollback
-- Generated on: 2026-05-02T20:50:14+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS tags_label_trgm_idx;
DROP INDEX IF EXISTS idx_tags_group_slug;
DROP INDEX IF EXISTS idx_tags_tenant_group;
DROP INDEX IF EXISTS idx_tags_tenant_id;
DROP INDEX IF EXISTS idx_tags_uuid;
DROP POLICY IF EXISTS tag_background_worker_access ON tags;
DROP POLICY IF EXISTS tag_isolation ON tags;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS tags CASCADE;
