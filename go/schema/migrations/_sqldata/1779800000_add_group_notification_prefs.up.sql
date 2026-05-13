-- Migration generated from schema differences
-- Generated on: 2026-05-13T10:37:35Z
-- Direction: UP

-- POSTGRES TABLE: group_notification_prefs --
CREATE TABLE group_notification_prefs (
  group_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  category TEXT NOT NULL,
  enabled BOOLEAN NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE group_notification_prefs ADD CONSTRAINT fk_group_notif_pref_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE group_notification_prefs ADD CONSTRAINT fk_group_notif_pref_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE group_notification_prefs ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- Enable RLS for group_notification_prefs table
ALTER TABLE group_notification_prefs ENABLE ROW LEVEL SECURITY;
-- Allows background workers to read per-group prefs when deciding whether to enqueue a reminder
DROP POLICY IF EXISTS group_notification_prefs_background_worker_access ON group_notification_prefs;
CREATE POLICY group_notification_prefs_background_worker_access ON group_notification_prefs FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Ensures per-group notification prefs are isolated by tenant; user-level filtering happens in application logic
DROP POLICY IF EXISTS group_notification_prefs_tenant_isolation ON group_notification_prefs;
CREATE POLICY group_notification_prefs_tenant_isolation ON group_notification_prefs FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
CREATE INDEX IF NOT EXISTS idx_group_notification_prefs_tenant_id ON group_notification_prefs (tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_notification_prefs_unique ON group_notification_prefs (tenant_id, group_id, user_id, category);
CREATE INDEX IF NOT EXISTS idx_group_notification_prefs_user_group ON group_notification_prefs (user_id, group_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_notification_prefs_uuid ON group_notification_prefs (uuid);