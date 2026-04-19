-- Migration generated from schema differences
-- Generated on: 2026-04-16T22:35:37+02:00
-- Direction: UP

-- Add/modify columns for table: invoices --
-- ALTER statements: --
ALTER TABLE invoices ADD COLUMN group_id TEXT;
-- Add/modify columns for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ADD COLUMN group_id TEXT;
-- Add/modify columns for table: locations --
-- ALTER statements: --
ALTER TABLE locations ADD COLUMN group_id TEXT;
-- Add/modify columns for table: manuals --
-- ALTER statements: --
ALTER TABLE manuals ADD COLUMN group_id TEXT;
-- Add/modify columns for table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps ADD COLUMN group_id TEXT;
-- Add/modify columns for table: exports --
-- ALTER statements: --
ALTER TABLE exports ADD COLUMN group_id TEXT;
-- Add/modify columns for table: files --
-- ALTER statements: --
ALTER TABLE files ADD COLUMN group_id TEXT;
-- Add/modify columns for table: areas --
-- ALTER statements: --
ALTER TABLE areas ADD COLUMN group_id TEXT;
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN group_id TEXT;
-- Add/modify columns for table: images --
-- ALTER statements: --
ALTER TABLE images ADD COLUMN group_id TEXT;
-- Add foreign key constraints for table: invoices --
-- ALTER statements: --
ALTER TABLE invoices ADD CONSTRAINT fk_invoice_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: restore_operations --
-- ALTER statements: --
ALTER TABLE restore_operations ADD CONSTRAINT fk_restore_operation_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: locations --
-- ALTER statements: --
ALTER TABLE locations ADD CONSTRAINT fk_location_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: manuals --
-- ALTER statements: --
ALTER TABLE manuals ADD CONSTRAINT fk_manual_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: restore_steps --
-- ALTER statements: --
ALTER TABLE restore_steps ADD CONSTRAINT fk_restore_step_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: exports --
-- ALTER statements: --
ALTER TABLE exports ADD CONSTRAINT fk_export_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: files --
-- ALTER statements: --
ALTER TABLE files ADD CONSTRAINT fk_file_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: areas --
-- ALTER statements: --
ALTER TABLE areas ADD CONSTRAINT fk_area_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD CONSTRAINT fk_commodity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- Add foreign key constraints for table: images --
-- ALTER statements: --
ALTER TABLE images ADD CONSTRAINT fk_image_group FOREIGN KEY (group_id) REFERENCES location_groups(id);