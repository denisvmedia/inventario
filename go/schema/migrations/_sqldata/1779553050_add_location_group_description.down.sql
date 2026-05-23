-- Migration rollback: drop the description column added by the companion
-- UP migration (issue #1647).
-- Direction: DOWN
--
-- CASCADE only matters if a future view / generated column references the
-- column — none today, but mirroring the locations/areas rollback pattern
-- from 1779500000_add_location_area_icon_description.down.sql.

ALTER TABLE location_groups DROP COLUMN description CASCADE;
