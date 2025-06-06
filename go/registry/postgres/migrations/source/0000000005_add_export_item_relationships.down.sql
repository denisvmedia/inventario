-- Remove relationship fields from export selected items
UPDATE exports 
SET selected_items = (
    SELECT jsonb_agg(
        item - 'location_id' - 'area_id'
    )
    FROM jsonb_array_elements(selected_items) AS item
)
WHERE selected_items IS NOT NULL 
AND jsonb_typeof(selected_items) = 'array'
AND selected_items != '[]'::jsonb;