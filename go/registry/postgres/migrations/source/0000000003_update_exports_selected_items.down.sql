-- Revert exports table selected_items column back to selected_item_ids
ALTER TABLE exports RENAME COLUMN selected_items TO selected_item_ids;