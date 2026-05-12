-- Migration rollback: drop the icon + description columns added by the
-- companion UP migration (issue #1531 item 4).
-- Direction: DOWN
--
-- CASCADE only matters if a future view / generated column references
-- these — none today, but mirroring the size_bytes rollback pattern
-- (1778223903_add_files_size_bytes.down.sql) for safety.

ALTER TABLE areas DROP COLUMN icon CASCADE;
ALTER TABLE locations DROP COLUMN description CASCADE;
ALTER TABLE locations DROP COLUMN icon CASCADE;
