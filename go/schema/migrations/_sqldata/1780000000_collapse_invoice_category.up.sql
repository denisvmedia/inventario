-- Migration: collapse FileCategoryInvoices into FileCategoryDocuments
-- and preserve the "invoice" semantic as a conventional tag (#1622).
-- Direction: UP
--
-- The `category` column is plain TEXT (no DB-side enum constraint), so
-- the Go-side enum collapse produces no schema diff — this migration is
-- pure data UPDATE. Three idempotent passes:
--
-- 1. Provision a `tags` row with slug='invoice' for every group that
--    has a file currently in the `invoices` category, so the JSONB
--    references the migration is about to write line up with the
--    tag-catalogue (the FE Tags page + autocomplete read from that
--    table). ON CONFLICT keeps reruns no-op once the row exists.
--
-- 2. Append `"invoice"` to every legacy invoice file's tags JSONB
--    array, deduplicated against whatever the row already carries.
--    Uses jsonb_set + COALESCE so a NULL tags column lands as
--    `["invoice"]` instead of NULL. Selection predicate excludes rows
--    whose tags already contain `"invoice"` so the array stays free
--    of duplicates.
--
-- 3. Flip category='invoices' → category='documents' across the same
--    row set. Once this passes, the predicate matches no rows; the
--    migration is idempotent across reruns.
--
-- All three passes refresh updated_at so audit logs / change-listeners
-- see the migration as a single bump.

-- Pass 1: provision tag rows --------------------------------------------------
-- We touch the `tags` table directly (RLS isn't relevant in migrations —
-- they run as the migrator role with bypassrls). The composite uniqueness
-- key is (tenant_id, group_id, slug); ON CONFLICT covers it without us
-- needing to name the constraint.
INSERT INTO tags (id, uuid, tenant_id, group_id, created_by_user_id,
                  slug, label, color, created_at, updated_at)
SELECT
    -- IDs are TEXT — gen_random_uuid()::text matches the format used
    -- by application-side inserts elsewhere in the schema.
    gen_random_uuid()::text,
    gen_random_uuid(),
    f.tenant_id,
    f.group_id,
    f.created_by_user_id,
    'invoice',
    'Invoice',
    'muted',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM files f
WHERE f.category = 'invoices'
GROUP BY f.tenant_id, f.group_id, f.created_by_user_id
ON CONFLICT (tenant_id, group_id, slug) DO NOTHING;

-- Pass 2: append "invoice" to the tags JSONB array (idempotent) --------------
UPDATE files
SET tags = COALESCE(tags, '[]'::jsonb) || '["invoice"]'::jsonb,
    updated_at = CURRENT_TIMESTAMP
WHERE category = 'invoices'
  AND NOT COALESCE(tags, '[]'::jsonb) @> '["invoice"]'::jsonb;

-- Pass 3: reclassify category -------------------------------------------------
UPDATE files
SET category = 'documents',
    updated_at = CURRENT_TIMESTAMP
WHERE category = 'invoices';
