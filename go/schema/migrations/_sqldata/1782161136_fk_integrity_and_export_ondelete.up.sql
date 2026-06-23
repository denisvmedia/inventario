-- Migration generated from schema differences
-- Generated on: 2026-06-22T20:45:36Z
-- Direction: UP

-- ALTER statements: --
ALTER TABLE commodities ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE locations ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE restore_operations ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE areas ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE restore_steps ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE user_mfa_secrets ADD CONSTRAINT fk_entity_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id);
-- ALTER statements: --
ALTER TABLE areas ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE exports ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE files ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE restore_operations ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE user_mfa_secrets ADD CONSTRAINT fk_entity_user FOREIGN KEY (user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE restore_steps ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- ALTER statements: --
ALTER TABLE exports ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE commodities ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE files ADD CONSTRAINT fk_entity_group FOREIGN KEY (group_id) REFERENCES location_groups(id);
-- ALTER statements: --
ALTER TABLE locations ADD CONSTRAINT fk_entity_created_by FOREIGN KEY (created_by_user_id) REFERENCES users(id);
-- Drop constraint fk_export_file (table resolved at runtime from information_schema)
DO $ptah$
DECLARE
    target_table TEXT;
BEGIN
    SELECT table_name INTO target_table
    FROM information_schema.table_constraints
    WHERE constraint_name = 'fk_export_file'
      AND table_schema = current_schema()
    LIMIT 1;

    IF target_table IS NOT NULL THEN
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', target_table, 'fk_export_file');
        RAISE NOTICE 'Dropped constraint fk_export_file from table %', target_table;
    ELSE
        RAISE NOTICE 'Constraint fk_export_file not found in current schema';
    END IF;
END
$ptah$;
-- ALTER statements: --
ALTER TABLE exports ADD CONSTRAINT fk_export_file FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE SET NULL;