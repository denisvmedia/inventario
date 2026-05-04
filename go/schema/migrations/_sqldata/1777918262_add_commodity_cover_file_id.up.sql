-- Migration generated from schema differences
-- Generated on: 2026-05-04T20:11:02+02:00
-- Direction: UP

-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE commodities ADD COLUMN cover_file_id TEXT;
-- Add foreign key constraints for table: commodities --
-- ALTER statements: --
-- ON DELETE SET NULL is added manually: the Ptah generator does not yet emit
-- delete behaviour even when the model carries on_delete="SET NULL". Keeping
-- it explicit here ensures that deleting a file silently clears the cover
-- override (the resolver's first-photo path takes over) instead of blocking
-- the file delete.
ALTER TABLE commodities ADD CONSTRAINT fk_commodity_cover_file FOREIGN KEY (cover_file_id) REFERENCES files(id) ON DELETE SET NULL;