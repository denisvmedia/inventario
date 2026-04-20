-- Migration generated from schema differences
-- Generated on: 2026-04-20T20:43:24+02:00
-- Direction: UP

-- Add/modify columns for table: users --
-- ALTER statements: --
ALTER TABLE users ADD COLUMN default_group_id TEXT;
-- Add foreign key constraints for table: users --
-- ALTER statements: --
-- ON DELETE SET NULL is added manually: the Ptah generator does not yet emit
-- delete behaviour even when the model carries on_delete="SET NULL". Keeping
-- it explicit here ensures that deleting a group clears every user's
-- default_group_id preference instead of blocking the delete.
ALTER TABLE users ADD CONSTRAINT fk_user_default_group FOREIGN KEY (default_group_id) REFERENCES location_groups(id) ON DELETE SET NULL;
