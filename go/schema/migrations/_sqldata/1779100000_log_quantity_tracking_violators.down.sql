-- Issue #1554: log-only migration — nothing to undo. The up path emits
-- PostgreSQL NOTICEs and writes no data, so a no-op down keeps the
-- rollback path consistent without fabricating state.
SELECT 1;
