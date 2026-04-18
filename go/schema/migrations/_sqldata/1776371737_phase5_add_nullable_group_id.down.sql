-- Migration rollback
-- Generated on: 2026-04-16T22:35:37+02:00
-- Direction: DOWN

-- Remove columns from table: invoices --
-- ALTER statements: --
ALTER TABLE invoices DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column invoices.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column restore_operations.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: locations --
-- ALTER statements: --
ALTER TABLE locations DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column locations.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: manuals --
-- ALTER statements: --
ALTER TABLE manuals DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column manuals.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column restore_steps.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: exports --
-- ALTER statements: --
ALTER TABLE exports DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column exports.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE files DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column files.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: areas --
-- ALTER statements: --
ALTER TABLE areas DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column areas.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE commodities DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column commodities.group_id with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: images --
-- ALTER statements: --
ALTER TABLE images DROP COLUMN group_id CASCADE;
-- WARNING: Dropping column images.group_id with CASCADE - This will delete data and dependent objects! --;