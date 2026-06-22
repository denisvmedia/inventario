-- Migration rollback
-- Generated on: 2026-06-22T20:45:36Z
-- Direction: DOWN

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
ALTER TABLE exports ADD CONSTRAINT fk_export_file FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE NO ACTION ON UPDATE NO ACTION;
-- ALTER statements: --
ALTER TABLE commodities DROP CONSTRAINT IF EXISTS fk_entity_created_by;
-- ALTER statements: --
ALTER TABLE locations DROP CONSTRAINT IF EXISTS fk_entity_group;
-- ALTER statements: --
ALTER TABLE restore_operations DROP CONSTRAINT IF EXISTS fk_entity_created_by;
-- ALTER statements: --
ALTER TABLE areas DROP CONSTRAINT IF EXISTS fk_entity_group;
-- ALTER statements: --
ALTER TABLE restore_steps DROP CONSTRAINT IF EXISTS fk_entity_group;
-- ALTER statements: --
ALTER TABLE user_mfa_secrets DROP CONSTRAINT IF EXISTS fk_entity_tenant;
-- ALTER statements: --
ALTER TABLE areas DROP CONSTRAINT IF EXISTS fk_entity_created_by;
-- ALTER statements: --
ALTER TABLE exports DROP CONSTRAINT IF EXISTS fk_entity_created_by;
-- ALTER statements: --
ALTER TABLE files DROP CONSTRAINT IF EXISTS fk_entity_created_by;
-- ALTER statements: --
ALTER TABLE restore_operations DROP CONSTRAINT IF EXISTS fk_entity_group;
-- ALTER statements: --
ALTER TABLE user_mfa_secrets DROP CONSTRAINT IF EXISTS fk_entity_user;
-- ALTER statements: --
ALTER TABLE restore_steps DROP CONSTRAINT IF EXISTS fk_entity_created_by;
-- ALTER statements: --
ALTER TABLE exports DROP CONSTRAINT IF EXISTS fk_entity_group;
-- ALTER statements: --
ALTER TABLE commodities DROP CONSTRAINT IF EXISTS fk_entity_group;
-- ALTER statements: --
ALTER TABLE files DROP CONSTRAINT IF EXISTS fk_entity_group;
-- ALTER statements: --
ALTER TABLE locations DROP CONSTRAINT IF EXISTS fk_entity_created_by;