-- Migration generated from schema differences
-- Generated on: 2026-07-12T15:15:44Z
-- Direction: UP

CREATE INDEX IF NOT EXISTS files_original_path_idx ON files (original_path);