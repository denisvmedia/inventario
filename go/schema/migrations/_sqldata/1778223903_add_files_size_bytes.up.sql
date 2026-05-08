-- Migration generated from schema differences
-- Generated on: 2026-05-08T07:05:03Z
-- Direction: UP

-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ADD COLUMN size_bytes BIGINT NOT NULL DEFAULT '0';