-- Migration generated from schema differences
-- Generated on: 2026-04-30T18:44:30+02:00
-- Direction: UP

-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ADD COLUMN category TEXT NOT NULL DEFAULT 'other';
CREATE INDEX IF NOT EXISTS idx_files_tenant_group_category ON files (tenant_id, group_id, category);
