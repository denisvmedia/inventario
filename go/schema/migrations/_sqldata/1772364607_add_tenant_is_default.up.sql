-- Migration generated from schema differences
-- Generated on: 2026-03-01T12:30:07+01:00
-- Direction: UP

-- Add/modify columns for table: tenants --
-- ALTER statements: --
ALTER TABLE tenants ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT 'false';
CREATE UNIQUE INDEX IF NOT EXISTS tenants_single_default_idx ON tenants (is_default) WHERE is_default = true;