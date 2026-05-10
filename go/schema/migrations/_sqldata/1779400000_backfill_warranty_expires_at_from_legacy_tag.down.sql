-- Migration: backfill commodities.warranty_expires_at from the legacy
-- "warranty:YYYY-MM-DD" tag convention (issue #1367 cleanup).
-- Direction: DOWN
--
-- The UP migration drains the legacy tag into the typed column and
-- strips the tag. There's no safe way to identify which rows had been
-- written via the legacy tag versus the typed field after the fact —
-- every post-#1535 row also writes to the typed column directly.
-- Reverting would spray a synthetic tag onto rows that never had one.
-- Make the down a no-op.

SELECT 1;
