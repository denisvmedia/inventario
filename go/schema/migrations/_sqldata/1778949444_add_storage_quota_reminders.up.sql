-- Migration generated from schema differences
-- Generated on: 2026-05-16T16:37:24Z
-- Direction: UP

-- POSTGRES TABLE: storage_quota_reminders --
CREATE TABLE storage_quota_reminders (
  threshold_percent INTEGER NOT NULL,
  sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  group_id TEXT NOT NULL,
  created_by_user_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE storage_quota_reminders ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE storage_quota_reminders ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE storage_quota_reminders ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Enable RLS for storage_quota_reminders table
ALTER TABLE storage_quota_reminders ENABLE ROW LEVEL SECURITY;
-- Allows background workers to record reminder emissions across all groups
DROP POLICY IF EXISTS storage_quota_reminder_background_worker_access ON storage_quota_reminders;
CREATE POLICY storage_quota_reminder_background_worker_access ON storage_quota_reminders FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures storage quota reminders are accessible only by their tenant and group
DROP POLICY IF EXISTS storage_quota_reminder_isolation ON storage_quota_reminders;
CREATE POLICY storage_quota_reminder_isolation ON storage_quota_reminders FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != '');
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_quota_reminders_group_threshold ON storage_quota_reminders (group_id, threshold_percent);
CREATE INDEX IF NOT EXISTS idx_storage_quota_reminders_tenant_id ON storage_quota_reminders (tenant_id);