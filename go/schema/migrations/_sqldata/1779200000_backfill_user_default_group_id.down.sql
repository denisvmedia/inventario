-- Migration: backfill users.default_group_id (issue #1592).
-- Direction: DOWN
--
-- The UP migration backfills NULL → membership for every existing user.
-- There is no safe way to identify which rows were touched after the fact,
-- and the original NULLs were exactly the rows the invariant disallows, so
-- a real revert would re-introduce the bug. Make the down a no-op.
SELECT 1;
