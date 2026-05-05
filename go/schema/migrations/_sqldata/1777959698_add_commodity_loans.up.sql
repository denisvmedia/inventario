-- Migration generated from schema differences
-- Generated on: 2026-05-05T07:41:38+02:00
-- Direction: UP

-- POSTGRES TABLE: commodity_loans --
CREATE TABLE commodity_loans (
  commodity_id TEXT NOT NULL,
  borrower_name TEXT NOT NULL,
  borrower_contact TEXT,
  borrower_note TEXT,
  lent_at TEXT NOT NULL,
  due_back_at TEXT,
  returned_at TEXT,
  reminder_sent_overdue BOOLEAN NOT NULL DEFAULT 'false',
  reminder_sent_due_soon BOOLEAN NOT NULL DEFAULT 'false',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
-- ON DELETE CASCADE is added manually: the Ptah generator does not yet
-- emit delete behaviour even when the model carries on_delete="CASCADE".
-- Keeping it explicit here ensures that deleting a commodity drops its
-- loan history cleanly (no orphan rows). Mirror of the manual fixup
-- already applied to fk_commodity_cover_file in
-- 1777918262_add_commodity_cover_file_id.up.sql.
ALTER TABLE commodity_loans ADD CONSTRAINT fk_commodity_loan_commodity FOREIGN KEY (commodity_id) REFERENCES commodities(id) ON DELETE CASCADE;
-- ALTER statements: --
ALTER TABLE commodity_loans ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE commodity_loans ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE commodity_loans ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Enable RLS for commodity_loans table
ALTER TABLE commodity_loans ENABLE ROW LEVEL SECURITY;
-- Allows background workers to access all commodity loans for processing
DROP POLICY IF EXISTS commodity_loan_background_worker_access ON commodity_loans;
CREATE POLICY commodity_loan_background_worker_access ON commodity_loans FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures commodity loans can only be accessed and modified by their tenant and group with required contexts
DROP POLICY IF EXISTS commodity_loan_isolation ON commodity_loans;
CREATE POLICY commodity_loan_isolation ON commodity_loans FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE INDEX IF NOT EXISTS idx_commodity_loans_active ON commodity_loans (group_id, due_back_at) WHERE returned_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_commodity_loans_commodity ON commodity_loans (commodity_id, lent_at);
CREATE INDEX IF NOT EXISTS idx_commodity_loans_due ON commodity_loans (due_back_at) WHERE returned_at IS NULL AND due_back_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_commodity_loans_tenant_group ON commodity_loans (tenant_id, group_id);
CREATE INDEX IF NOT EXISTS idx_commodity_loans_tenant_id ON commodity_loans (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_commodity_loans_uuid ON commodity_loans (uuid);