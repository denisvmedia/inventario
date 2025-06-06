-- Add relationship fields to export selected items
-- This migration adds location_id and area_id fields to preserve hierarchical relationships
-- For existing exports, we cannot populate these fields since the original relationships
-- may no longer exist in the database

-- Update selected_items to add relationship fields for each item
UPDATE exports 
SET selected_items = (
    SELECT jsonb_agg(
        item || jsonb_build_object(
            'location_id', COALESCE(item->>'location_id', ''),
            'area_id', COALESCE(item->>'area_id', '')
        )
    )
    FROM jsonb_array_elements(selected_items) AS item
)
WHERE selected_items IS NOT NULL 
AND jsonb_typeof(selected_items) = 'array'
AND selected_items != '[]'::jsonb;