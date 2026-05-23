-- Migration rollback
-- Generated on: 2026-05-23T08:47:26Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_commodity_scan_audits_tenant_created;
DROP INDEX IF EXISTS idx_commodity_scan_audits_user_created;
DROP INDEX IF EXISTS idx_commodity_scan_audits_uuid;
-- Drop RLS policy commodity_scan_audit_background_worker_access from table commodity_scan_audits
DROP POLICY IF EXISTS commodity_scan_audit_background_worker_access ON commodity_scan_audits;
-- Drop RLS policy commodity_scan_audit_isolation from table commodity_scan_audits
DROP POLICY IF EXISTS commodity_scan_audit_isolation ON commodity_scan_audits;
-- NOTE: RLS policies were removed from table commodity_scan_audits - verify if RLS should be disabled --
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS commodity_scan_audits CASCADE;