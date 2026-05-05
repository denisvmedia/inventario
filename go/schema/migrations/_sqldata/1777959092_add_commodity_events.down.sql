-- Migration rollback
-- Generated on: 2026-05-05T07:31:32+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS commodity_events_kind_idx;
DROP INDEX IF EXISTS commodity_events_lookup;
DROP INDEX IF EXISTS idx_commodity_events_tenant_group;
DROP INDEX IF EXISTS idx_commodity_events_tenant_id;
DROP INDEX IF EXISTS idx_commodity_events_uuid;
-- Drop RLS policy commodity_event_background_worker_access from table commodity_events
DROP POLICY IF EXISTS commodity_event_background_worker_access ON commodity_events;
-- Drop RLS policy commodity_event_isolation from table commodity_events
DROP POLICY IF EXISTS commodity_event_isolation ON commodity_events;
-- NOTE: RLS policies were removed from table commodity_events - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS commodity_events CASCADE;