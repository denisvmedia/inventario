-- Migration rollback
-- Generated on: 2026-05-02T22:54:56+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS idx_tags_group_slug;
DROP INDEX IF EXISTS idx_tags_tenant_group;
DROP INDEX IF EXISTS idx_tags_tenant_id;
DROP INDEX IF EXISTS idx_tags_uuid;
DROP INDEX IF EXISTS tags_label_trgm_idx;
-- Drop RLS policy tag_background_worker_access from table tags
DROP POLICY IF EXISTS tag_background_worker_access ON tags;
-- Drop RLS policy tag_isolation from table tags
DROP POLICY IF EXISTS tag_isolation ON tags;
-- NOTE: RLS policies were removed from table tags - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS tags CASCADE;