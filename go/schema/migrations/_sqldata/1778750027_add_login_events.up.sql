-- Migration generated from schema differences
-- Generated on: 2026-05-14T09:13:47Z
-- Direction: UP

-- POSTGRES TABLE: login_events --
CREATE TABLE login_events (
  user_id TEXT,
  email TEXT NOT NULL,
  outcome TEXT NOT NULL,
  method TEXT NOT NULL DEFAULT 'password',
  ip_address VARCHAR(64),
  user_agent TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  tenant_id TEXT NOT NULL,
  id TEXT PRIMARY KEY NOT NULL,
  uuid TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- ALTER statements: --
ALTER TABLE login_events ADD CONSTRAINT fk_login_event_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE login_events ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- Enable RLS for login_events table
ALTER TABLE login_events ENABLE ROW LEVEL SECURITY;
-- Allows the retention worker and login flow to insert/sweep events outside any user context
DROP POLICY IF EXISTS login_event_background_worker_access ON login_events;
CREATE POLICY login_event_background_worker_access ON login_events FOR ALL TO inventario_background_worker
    USING (true)
    WITH CHECK (true);
-- Login events are tenant-isolated; per-user filtering happens in application logic so a user only sees their own attempts
DROP POLICY IF EXISTS login_event_tenant_isolation ON login_events;
CREATE POLICY login_event_tenant_isolation ON login_events FOR ALL TO inventario_app
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '');
CREATE INDEX IF NOT EXISTS idx_login_events_created_at ON login_events (created_at);
CREATE INDEX IF NOT EXISTS idx_login_events_tenant_id ON login_events (tenant_id);
CREATE INDEX IF NOT EXISTS idx_login_events_user_created_at ON login_events (user_id, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_login_events_uuid ON login_events (uuid);