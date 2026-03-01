-- Migration generated from schema differences
-- Generated on: 2026-03-01T12:30:07+01:00
-- Direction: UP

-- Add/modify columns for table: tenants --
-- ALTER statements: --
ALTER TABLE tenants ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT 'false';
CREATE UNIQUE INDEX IF NOT EXISTS tenants_single_default_idx ON tenants (is_default) WHERE is_default = true;

-- Mark the sole pre-existing tenant as the system default when none is designated yet.
-- This backfills the is_default flag that was missing before this column existed.
-- Safety: only acts when exactly one tenant exists and none has is_default = true (idempotent).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'tenants' AND column_name = 'is_default' AND table_schema = 'public') THEN
        IF NOT EXISTS (SELECT 1 FROM tenants WHERE is_default = true) AND (SELECT COUNT(*) FROM tenants) = 1 THEN
            UPDATE tenants SET is_default = true;
            RAISE NOTICE 'Marked sole tenant as system default';
        ELSE
            RAISE NOTICE 'Default tenant already set or multiple tenants exist - skipping';
        END IF;
    END IF;
END $$;