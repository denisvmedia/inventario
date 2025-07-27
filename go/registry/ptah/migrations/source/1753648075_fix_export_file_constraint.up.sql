-- Migration generated from schema differences
-- Generated on: 2025-07-27T22:27:55+02:00
-- Direction: UP

-- Modify table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ALTER COLUMN binary_data_size TYPE BIGINT;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE restore_operations ALTER COLUMN binary_data_size SET DEFAULT '0';
-- Modify column restore_operations.binary_data_size: default_expr: '0'::bigint -> 0 --
-- Modify table: exports --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN binary_data_size TYPE BIGINT;
ALTER TABLE exports ALTER COLUMN binary_data_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN binary_data_size SET DEFAULT '0';
-- Modify column exports.binary_data_size: default_expr: '0'::bigint -> 0 --
-- ALTER statements: --
ALTER TABLE exports ALTER COLUMN file_size TYPE BIGINT;
ALTER TABLE exports ALTER COLUMN file_size DROP NOT NULL;
ALTER TABLE exports ALTER COLUMN file_size SET DEFAULT '0';
-- Modify column exports.file_size: default_expr: '0'::bigint -> 0 --;

-- Fix foreign key constraint for export file relationship
-- Change from no action to SET NULL on delete to allow file deletion
-- Drop the existing constraint
ALTER TABLE exports DROP CONSTRAINT IF EXISTS fk_export_file;
-- Recreate the constraint with ON DELETE SET NULL
ALTER TABLE exports ADD CONSTRAINT fk_export_file
    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE SET NULL;