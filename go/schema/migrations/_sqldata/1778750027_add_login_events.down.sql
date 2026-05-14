-- Migration rollback
-- Generated on: 2026-05-14T09:13:47Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_login_events_created_at;
DROP INDEX IF EXISTS idx_login_events_tenant_id;
DROP INDEX IF EXISTS idx_login_events_user_created_at;
DROP INDEX IF EXISTS idx_login_events_uuid;
-- Drop RLS policy login_event_background_worker_access from table login_events
DROP POLICY IF EXISTS login_event_background_worker_access ON login_events;
-- Drop RLS policy login_event_tenant_isolation from table login_events
DROP POLICY IF EXISTS login_event_tenant_isolation ON login_events;
-- NOTE: RLS policies were removed from table login_events - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS login_events CASCADE;