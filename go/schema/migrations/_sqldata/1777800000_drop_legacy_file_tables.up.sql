-- Drop legacy commodity-scoped attachment tables.
--
-- Issue #1421 phase 2: the unified `files` table (introduced in #1397 and
-- backfilled by #1399) replaces the per-bucket images / invoices / manuals
-- tables. The PR that owns this migration (a) backfills any remaining rows
-- via `inventario migrate filesbackfill --apply` (which itself is gone after
-- this PR — operators must run the previous binary if they have not yet
-- backfilled), then (b) drops the legacy tables.
--
-- Pre-condition: ops MUST have run `inventario migrate filesbackfill --apply`
-- on every production environment before applying this migration. The down
-- migration recreates empty tables so an immediate rollback succeeds, but
-- their previous contents are NOT restored.

DROP TABLE IF EXISTS images CASCADE;
DROP TABLE IF EXISTS invoices CASCADE;
DROP TABLE IF EXISTS manuals CASCADE;
