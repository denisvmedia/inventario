-- Update exports table to use new selected_items structure instead of selected_item_ids
-- This migration renames the column to maintain data compatibility
ALTER TABLE exports RENAME COLUMN selected_item_ids TO selected_items;
