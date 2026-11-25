-- Migration rollback
-- Generated on: 2026-11-25T08:00:50+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Remove columns from table: maintenance_reminders --
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" DROP COLUMN "silent" CASCADE;
-- WARNING: Dropping column maintenance_reminders.silent with CASCADE - This will delete data and dependent objects! --
-- Remove columns from table: warranty_reminders --
-- ALTER statements: --
ALTER TABLE "warranty_reminders" DROP COLUMN "silent" CASCADE;
-- WARNING: Dropping column warranty_reminders.silent with CASCADE - This will delete data and dependent objects! --;