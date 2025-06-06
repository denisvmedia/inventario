-- Add name field to export selected items
-- For existing exports, we'll populate the name field with the item ID as a fallback
-- since we cannot retrieve the original names for potentially deleted items

-- Update selected_items to add name field for each item
UPDATE exports 
SET selected_items = (
    SELECT jsonb_agg(
        CASE 
            WHEN item->>'name' IS NULL THEN 
                item || jsonb_build_object('name', '[Item ' || (item->>'id') || ']')
            ELSE item
        END
    )
    FROM jsonb_array_elements(selected_items) AS item
)
WHERE selected_items IS NOT NULL 
AND jsonb_typeof(selected_items) = 'array'
AND selected_items != '[]'::jsonb;
