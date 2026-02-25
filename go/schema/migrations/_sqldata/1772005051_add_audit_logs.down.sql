-- Migration rollback
-- Generated on: 2026-02-25T08:37:31+01:00
-- Direction: DOWN

DROP INDEX IF EXISTS audit_logs_action_idx;
DROP INDEX IF EXISTS audit_logs_entity_idx;
DROP INDEX IF EXISTS audit_logs_tenant_id_idx;
DROP INDEX IF EXISTS audit_logs_timestamp_idx;
DROP INDEX IF EXISTS audit_logs_user_id_idx;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS audit_logs CASCADE;