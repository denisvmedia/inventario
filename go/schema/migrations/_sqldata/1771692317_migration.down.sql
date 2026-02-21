-- Migration rollback: operation_slots table
-- Direction: DOWN

DROP INDEX IF EXISTS idx_operation_slots_cleanup;
DROP INDEX IF EXISTS idx_operation_slots_operation;
DROP INDEX IF EXISTS idx_operation_slots_unique;
DROP INDEX IF EXISTS idx_operation_slots_user_operation;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS operation_slots CASCADE;
