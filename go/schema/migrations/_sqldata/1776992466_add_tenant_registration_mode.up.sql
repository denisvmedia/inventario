-- Migration generated from schema differences
-- Generated on: 2026-04-24T03:01:06+02:00
-- Direction: UP

-- Add/modify columns for table: tenants --
-- ALTER statements: --
ALTER TABLE tenants ADD COLUMN registration_mode TEXT NOT NULL DEFAULT 'closed';