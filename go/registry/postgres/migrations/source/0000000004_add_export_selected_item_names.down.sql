-- Remove name field from export selected items
-- This removes the name field from all selected items

UPDATE exports 
SET selected_items = (
    SELECT jsonb_agg(item - 'name')
    FROM jsonb_array_elements(selected_items) AS item
)
WHERE selected_items IS NOT NULL 
AND jsonb_typeof(selected_items) = 'array'
AND selected_items != '[]'::jsonb;
