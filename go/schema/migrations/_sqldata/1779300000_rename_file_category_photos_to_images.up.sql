-- Migration: rename FileCategory enum value `photos` → `images`.
-- Direction: UP
--
-- The Go-side `FileCategory` enum used `photos` as the bucket-name for
-- image files. The design contract (`design-mocks/`) names the bucket
-- `images`; the linked-entity-meta side already used `images` for the
-- commodity images bucket, so renaming the FileCategory value to `images`
-- aligns BE + FE + design-mock terminology in one move.
--
-- The `category` column is plain TEXT (no DB-side enum constraint), so a
-- model-annotation diff produces no schema change — this migration is a
-- pure data UPDATE. Idempotent: re-running selects no rows once every
-- old `photos` value has been rewritten to `images`.

UPDATE files SET category = 'images', updated_at = CURRENT_TIMESTAMP WHERE category = 'photos';
