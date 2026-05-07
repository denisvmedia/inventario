-- Migration rollback for 1779000000_add_currency_migrations
-- Direction: DOWN

DROP INDEX IF EXISTS idx_currency_migration_audit_uuid;
DROP INDEX IF EXISTS idx_currency_migration_audit_migration;
DROP INDEX IF EXISTS idx_currency_migration_audit_commodity;
DROP INDEX IF EXISTS idx_currency_migration_audit_tenant_group;
DROP INDEX IF EXISTS idx_currency_migrations_uuid;
DROP INDEX IF EXISTS idx_currency_migrations_tenant_group;
DROP INDEX IF EXISTS idx_currency_migrations_group_status;
DROP INDEX IF EXISTS idx_currency_migrations_group_completed;
DROP INDEX IF EXISTS idx_currency_migrations_group_in_flight;

DROP POLICY IF EXISTS currency_migration_audit_isolation ON currency_migration_audit_rows;
DROP POLICY IF EXISTS currency_migration_audit_background_worker_access ON currency_migration_audit_rows;
DROP POLICY IF EXISTS currency_migration_isolation ON currency_migrations;
DROP POLICY IF EXISTS currency_migration_background_worker_access ON currency_migrations;

ALTER TABLE location_groups DROP CONSTRAINT IF EXISTS fk_location_group_currency_migration;
ALTER TABLE location_groups DROP COLUMN IF EXISTS currency_migration_id CASCADE;

ALTER TABLE commodities DROP COLUMN IF EXISTS acquisition_currency CASCADE;
ALTER TABLE commodities DROP COLUMN IF EXISTS acquisition_price CASCADE;

DROP TABLE IF EXISTS currency_migration_audit_rows CASCADE;
DROP TABLE IF EXISTS currency_migrations CASCADE;
