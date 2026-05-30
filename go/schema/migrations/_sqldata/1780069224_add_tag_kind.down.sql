-- Migration rollback
-- Generated on: 2026-05-29T15:40:24Z
-- Direction: DOWN

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_group_slug ON tags (group_id, slug);
DROP INDEX IF EXISTS idx_tags_group_kind_slug;
-- Remove columns from table: tags --
-- ALTER statements: --
ALTER TABLE tags DROP COLUMN kind CASCADE;
-- WARNING: Dropping column tags.kind with CASCADE - This will delete data and dependent objects! --;