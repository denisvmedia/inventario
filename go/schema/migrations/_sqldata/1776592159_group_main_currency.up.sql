-- Migration generated from schema differences
-- Generated on: 2026-04-19T11:49:19+02:00
-- Direction: UP

-- Add/modify columns for table: location_groups --
-- ALTER statements: --
ALTER TABLE location_groups ADD COLUMN main_currency TEXT NOT NULL DEFAULT 'USD';