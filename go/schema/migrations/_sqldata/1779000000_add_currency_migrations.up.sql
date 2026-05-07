-- Migration: add currency migration scaffolding (issue #1550 / epic #202).
-- Direction: UP
--
-- This PR ships only the schema, models, and registries. No behaviour
-- change yet: nothing writes to currency_migrations / currency_migration_audit_rows,
-- and the new commodity columns / location_group column stay NULL because
-- the migration worker is introduced in PR 3 and the API endpoints in PR 2.

-- POSTGRES TABLE: currency_migrations --
CREATE TABLE currency_migrations (
  status TEXT NOT NULL,
  from_currency TEXT NOT NULL,
  to_currency TEXT NOT NULL,
  exchange_rate DECIMAL(20,10) NOT NULL,
  commodity_count INTEGER NOT NULL DEFAULT 0,
  total_before DECIMAL(20,2),
  total_after DECIMAL(20,2),
  preview_token TEXT,
  preview_expires_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  started_at TIMESTAMP,
  completed_at TIMESTAMP,
  error_message TEXT,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
ALTER TABLE currency_migrations ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE currency_migrations ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
ALTER TABLE currency_migrations ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- NOTE: a CHECK (from_currency <> to_currency) constraint would be a useful
-- schema-level guard, but ptah's walker.go currently drops table-level
-- Database.Constraints when accumulating per-file ParseFS results (the
-- migration drift checker would emit a spurious DROP CONSTRAINT every
-- time). Same-currency rows are still rejected at the apiserver layer
-- (422) and inside CurrencyMigration.ValidateWithContext. Re-add when
-- upstream walker is fixed.

-- POSTGRES TABLE: currency_migration_audit_rows --
-- Per-commodity before/after image, kept forever. commodity_id is
-- ON DELETE SET NULL so the audit survives commodity deletion.
CREATE TABLE currency_migration_audit_rows (
  migration_id TEXT NOT NULL,
  commodity_id TEXT,
  original_price_before DECIMAL(15,2),
  original_price_after DECIMAL(15,2),
  original_currency_before TEXT,
  original_currency_after TEXT,
  converted_before DECIMAL(15,2),
  converted_after DECIMAL(15,2),
  current_before DECIMAL(15,2),
  current_after DECIMAL(15,2),
  acquisition_filled_in_this_run BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
ALTER TABLE currency_migration_audit_rows ADD CONSTRAINT fk_currency_migration_audit_migration FOREIGN KEY (migration_id) REFERENCES currency_migrations(id) ON DELETE CASCADE;
ALTER TABLE currency_migration_audit_rows ADD CONSTRAINT fk_currency_migration_audit_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id) ON DELETE SET NULL;
ALTER TABLE currency_migration_audit_rows ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
ALTER TABLE currency_migration_audit_rows ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
ALTER TABLE currency_migration_audit_rows ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);

-- New columns on commodities for the as-purchased provenance pair --
ALTER TABLE commodities ADD COLUMN acquisition_price DECIMAL(15,2);
ALTER TABLE commodities ADD COLUMN acquisition_currency TEXT;
-- NOTE: a CHECK ((acquisition_price IS NULL) = (acquisition_currency IS NULL))
-- constraint would be a useful schema-level pair invariant, but ptah's
-- walker.go currently drops Database.Constraints from per-file
-- ParseFS results — it would drift vs every check run. The pair is
-- enforced at the application layer instead: migrationops.SetAcquisition
-- (the only writer) writes both columns atomically; CommodityRegistry
-- Create/Update drop user-supplied values + preserve existing ones.

-- New column on location_groups: in-flight migration lock signal. NULL
-- whenever no migration is running for the group. Read-only on JSON:API.
-- The FK is added below after currency_migrations exists.
ALTER TABLE location_groups ADD COLUMN currency_migration_id TEXT;
ALTER TABLE location_groups ADD CONSTRAINT fk_location_group_currency_migration FOREIGN KEY (currency_migration_id) REFERENCES currency_migrations(id) ON DELETE SET NULL;

-- RLS for currency_migrations --
ALTER TABLE currency_migrations ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS currency_migration_isolation ON currency_migrations;
CREATE POLICY currency_migration_isolation ON currency_migrations FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
DROP POLICY IF EXISTS currency_migration_background_worker_access ON currency_migrations;
CREATE POLICY currency_migration_background_worker_access ON currency_migrations FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);

-- RLS for currency_migration_audit_rows --
ALTER TABLE currency_migration_audit_rows ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS currency_migration_audit_isolation ON currency_migration_audit_rows;
CREATE POLICY currency_migration_audit_isolation ON currency_migration_audit_rows FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
DROP POLICY IF EXISTS currency_migration_audit_background_worker_access ON currency_migration_audit_rows;
CREATE POLICY currency_migration_audit_background_worker_access ON currency_migration_audit_rows FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);

-- Indexes on currency_migrations --
CREATE UNIQUE INDEX IF NOT EXISTS idx_currency_migrations_uuid ON currency_migrations (uuid);
CREATE INDEX IF NOT EXISTS idx_currency_migrations_tenant_group ON currency_migrations (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_currency_migrations_group_status ON currency_migrations (group_id, status);
-- Daily-cap query: SELECT count(*) WHERE group_id = ? AND status='completed' AND completed_at >= today.
-- Partial index keeps the scan tight regardless of historical volume.
CREATE INDEX IF NOT EXISTS idx_currency_migrations_group_completed
    ON currency_migrations (group_id, completed_at)
    WHERE status = 'completed';
-- Schema-level guard against the simultaneous-start race: two parallel
-- start handlers cannot each insert a pending row, the second one trips
-- the unique-violation. The registry maps SQLState 23505 (unique_violation)
-- on this index name to ErrMigrationInFlight which the API surfaces as
-- 409 currency_migration.migration_in_progress.
CREATE UNIQUE INDEX IF NOT EXISTS idx_currency_migrations_group_in_flight
    ON currency_migrations (group_id)
    WHERE status IN ('pending', 'running');

-- Indexes on currency_migration_audit_rows --
CREATE UNIQUE INDEX IF NOT EXISTS idx_currency_migration_audit_uuid ON currency_migration_audit_rows (uuid);
CREATE INDEX IF NOT EXISTS idx_currency_migration_audit_migration ON currency_migration_audit_rows (migration_id);
CREATE INDEX IF NOT EXISTS idx_currency_migration_audit_commodity ON currency_migration_audit_rows (commodity_id);
CREATE INDEX IF NOT EXISTS idx_currency_migration_audit_tenant_group ON currency_migration_audit_rows (tenant_id, group_id);
