-- Migration generated from schema differences
-- Generated on: 2026-05-03T15:46:26+02:00
-- Direction: UP

-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ADD COLUMN file_count INTEGER DEFAULT '0';

-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ADD COLUMN file_count INTEGER DEFAULT '0';
