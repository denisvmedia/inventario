-- Migration: backfill commodities.warranty_expires_at from the legacy
-- "warranty:YYYY-MM-DD" tag convention (issue #1367 cleanup).
-- Direction: UP
--
-- Pre-#1535, warranty expiry was carried as a tag matching
-- ^warranty:[0-9]{4}-[0-9]{2}-[0-9]{2}$ on the commodity row. The
-- first-class column landed in #1535. This migration drains the old
-- tags into the typed column and strips them so the FE can drop its
-- dual-source warrantyStatus()/effectiveWarrantyExpiry() fallback.
--
-- Two passes, both idempotent (re-running selects no rows once the
-- invariant holds):
--
-- 1. Backfill: for every row where warranty_expires_at is unset
--    (NULL or empty string) AND tags contains a matching tag, copy
--    the lexicographically-first matching tag's date suffix into the
--    column. YYYY-MM-DD sorts naturally so this is also the
--    chronologically-earliest expiry across competing tags. Multiple
--    competing tags are vanishingly rare in practice and the loser
--    is dropped in pass 2 anyway.
--
-- 2. Strip: from every row whose tags contains a matching tag,
--    remove every "warranty:YYYY-MM-DD" entry. This also cleans up
--    rows that already had warranty_expires_at set but carried a
--    leftover legacy tag from before #1535.

WITH legacy AS (
    SELECT
        c.id,
        substring(picked.tag FROM 10) AS expires_at
    FROM commodities c
    CROSS JOIN LATERAL (
        SELECT j.tag
        FROM jsonb_array_elements_text(COALESCE(c.tags, '[]'::jsonb)) AS j(tag)
        WHERE j.tag ~ '^warranty:[0-9]{4}-[0-9]{2}-[0-9]{2}$'
        ORDER BY j.tag ASC
        LIMIT 1
    ) AS picked
    -- Pre-filter on the JSONB GIN index (commodities_tags_gin_idx) so
    -- only rows whose tags can possibly carry a legacy warranty entry
    -- run the LATERAL scan. Without this, every NULL/empty-warranty
    -- row pays the array-elements unfold cost even when its tags are
    -- empty or unrelated.
    WHERE (c.warranty_expires_at IS NULL OR c.warranty_expires_at = '')
      AND c.tags @? '$[*] ? (@ like_regex "^warranty:[0-9]{4}-[0-9]{2}-[0-9]{2}$")'
)
UPDATE commodities c
SET warranty_expires_at = legacy.expires_at
FROM legacy
WHERE c.id = legacy.id;

UPDATE commodities c
SET tags = COALESCE(
        (
            SELECT jsonb_agg(j.tag ORDER BY j.tag)
            FROM jsonb_array_elements_text(COALESCE(c.tags, '[]'::jsonb)) AS j(tag)
            WHERE j.tag !~ '^warranty:[0-9]{4}-[0-9]{2}-[0-9]{2}$'
        ),
        '[]'::jsonb
    )
WHERE c.tags @? '$[*] ? (@ like_regex "^warranty:[0-9]{4}-[0-9]{2}-[0-9]{2}$")';
