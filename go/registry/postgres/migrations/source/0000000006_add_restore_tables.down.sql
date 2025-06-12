-- Drop indexes first
DROP INDEX IF EXISTS idx_restore_steps_created_date;
DROP INDEX IF EXISTS idx_restore_steps_result;
DROP INDEX IF EXISTS idx_restore_steps_operation_id;

DROP INDEX IF EXISTS idx_restore_operations_created_date;
DROP INDEX IF EXISTS idx_restore_operations_status;
DROP INDEX IF EXISTS idx_restore_operations_export_id;

-- Drop tables (order matters due to foreign key constraints)
DROP TABLE IF EXISTS restore_steps;
DROP TABLE IF EXISTS restore_operations;
