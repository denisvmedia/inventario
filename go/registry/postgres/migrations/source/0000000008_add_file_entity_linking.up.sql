-- Add entity linking fields to files table
ALTER TABLE files ADD COLUMN linked_entity_type TEXT DEFAULT '';
ALTER TABLE files ADD COLUMN linked_entity_id TEXT DEFAULT '';
ALTER TABLE files ADD COLUMN linked_entity_meta TEXT DEFAULT '';

-- Create indexes for better query performance on linked entities
CREATE INDEX IF NOT EXISTS idx_files_linked_entity_type ON files(linked_entity_type);
CREATE INDEX IF NOT EXISTS idx_files_linked_entity_id ON files(linked_entity_id);
CREATE INDEX IF NOT EXISTS idx_files_linked_entity_type_id ON files(linked_entity_type, linked_entity_id);
