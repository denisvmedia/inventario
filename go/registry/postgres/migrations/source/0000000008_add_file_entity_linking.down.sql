-- Remove indexes
DROP INDEX IF EXISTS idx_files_linked_entity_type_id;
DROP INDEX IF EXISTS idx_files_linked_entity_id;
DROP INDEX IF EXISTS idx_files_linked_entity_type;

-- Remove entity linking fields from files table
ALTER TABLE files DROP COLUMN IF EXISTS linked_entity_meta;
ALTER TABLE files DROP COLUMN IF EXISTS linked_entity_id;
ALTER TABLE files DROP COLUMN IF EXISTS linked_entity_type;
