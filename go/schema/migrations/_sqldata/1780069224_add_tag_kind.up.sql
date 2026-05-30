-- Migration generated from schema differences
-- Generated on: 2026-05-29T15:40:24Z
-- Direction: UP

-- Add/modify columns for table: tags --
-- ALTER statements: --
ALTER TABLE tags ADD COLUMN kind TEXT NOT NULL DEFAULT 'commodity';
CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_group_kind_slug ON tags (group_id, kind, slug);
DROP INDEX IF EXISTS idx_tags_group_slug;