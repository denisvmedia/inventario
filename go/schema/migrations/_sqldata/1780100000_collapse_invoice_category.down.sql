-- Migration: collapse FileCategoryInvoices into FileCategoryDocuments
-- and preserve the "invoice" semantic as a conventional tag (#1622).
-- Direction: DOWN
--
-- Per the issue acceptance criteria: restore category='invoices' on
-- every row currently in `documents` AND carrying the `invoice` tag.
-- This is intentionally lossy in one direction — any document the
-- user explicitly tagged `invoice` post-migration (e.g. a contract
-- with an `invoice` annotation) will be reclassified back to
-- `invoices` on rollback. The alternative — refusing to flip without
-- a per-row marker — would force a separate "is_legacy_invoice"
-- column just to support a rollback that should be rare in practice.
--
-- The `invoice` tag itself is NOT stripped: the FE still surfaces it
-- as a clickable filter chip post-rollback, and stripping would
-- destroy user-authored tag data.
--
-- The `tags` rows created on UP are also left in place — they sit
-- alongside any user-curated tags and don't depend on FileCategory.

UPDATE files
SET category = 'invoices',
    updated_at = CURRENT_TIMESTAMP
WHERE category = 'documents'
  AND COALESCE(tags, '[]'::jsonb) @> '["invoice"]'::jsonb;
