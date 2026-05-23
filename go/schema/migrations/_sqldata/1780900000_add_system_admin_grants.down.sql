-- Migration rollback
-- Generated on: 2026-05-23T08:32:34Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_system_admin_grants_uuid;
DROP INDEX IF EXISTS system_admin_grants_user_id_idx;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS system_admin_grants CASCADE;