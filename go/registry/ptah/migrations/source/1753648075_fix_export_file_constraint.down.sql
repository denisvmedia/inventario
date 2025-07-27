-- Migration rollback
-- Generated on: 2025-07-27T22:27:55+02:00
-- Direction: DOWN

-- Modify table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ALTER COLUMN binary_data_size TYPE bigint;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size SET DEFAULT ''0'::bigint';
-- Modify column restore_operations.binary_data_size: default_expr: 0 -> '0'::bigint --
-- Modify table: exports --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN binary_data_size TYPE bigint;
ALTER TABLE exports ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN binary_data_size SET DEFAULT ''0'::bigint';
-- Modify column exports.binary_data_size: default_expr: 0 -> '0'::bigint --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN file_size TYPE bigint;
ALTER TABLE exports ALTER COLUMN file_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN file_size SET DEFAULT ''0'::bigint';
-- Modify column exports.file_size: default_expr: 0 -> '0'::bigint --;

-- Rollback foreign key constraint fix for export file relationship
-- Change back from SET NULL to no action on delete
-- Drop the constraint with SET NULL
ALTER TABLE exports DROP CONSTRAINT IF EXISTS fk_export_file;
-- Recreate the original constraint without ON DELETE SET NULL
ALTER TABLE exports ADD CONSTRAINT fk_export_file
    FOREIGN KEY (file_id) REFERENCES files(id);