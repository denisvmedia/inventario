-- Migration rollback
-- Generated on: 2026-05-30T08:06:35Z
-- Direction: DOWN

DROP INDEX IF EXISTS idx_worker_control_uuid;
DROP INDEX IF EXISTS worker_control_worker_type_idx;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS worker_control CASCADE;