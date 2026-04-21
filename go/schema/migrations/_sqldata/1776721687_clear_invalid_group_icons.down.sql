-- Issue #1255: data-only migration — nothing to undo. The original free-text
-- values are unrecoverable once overwritten; a no-op down keeps the rollback
-- path consistent with the rest of the migration stack without fabricating
-- data.
SELECT 1;
