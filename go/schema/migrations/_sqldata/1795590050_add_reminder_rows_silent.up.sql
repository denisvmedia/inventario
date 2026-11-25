-- Migration generated from schema differences
-- Generated on: 2026-11-25T08:00:50+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Add/modify columns for table: maintenance_reminders --
-- ALTER statements: --
ALTER TABLE "maintenance_reminders" ADD COLUMN "silent" BOOLEAN NOT NULL DEFAULT false;
-- Add/modify columns for table: warranty_reminders --
-- ALTER statements: --
ALTER TABLE "warranty_reminders" ADD COLUMN "silent" BOOLEAN NOT NULL DEFAULT false;