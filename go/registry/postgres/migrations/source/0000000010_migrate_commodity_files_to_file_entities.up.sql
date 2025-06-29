-- Migrate existing commodity files (images, manuals, invoices) to the generic file entity system

-- Migrate images to file entities
INSERT INTO files (
    id, 
    title, 
    description, 
    type, 
    tags, 
    path, 
    original_path, 
    ext, 
    mime_type, 
    linked_entity_type, 
    linked_entity_id, 
    linked_entity_meta, 
    created_at, 
    updated_at
)
SELECT 
    i.id,
    COALESCE(NULLIF(i.path, ''), i.original_path) as title, -- Use path as title, fallback to original_path
    '' as description, -- Empty description
    'image' as type,
    '[]'::jsonb as tags, -- Empty tags array
    i.path,
    i.original_path,
    i.ext,
    i.mime_type,
    'commodity' as linked_entity_type,
    i.commodity_id as linked_entity_id,
    'images' as linked_entity_meta,
    NOW() as created_at,
    NOW() as updated_at
FROM images i
WHERE NOT EXISTS (
    SELECT 1 FROM files f 
    WHERE f.id = i.id
);

-- Migrate manuals to file entities
INSERT INTO files (
    id, 
    title, 
    description, 
    type, 
    tags, 
    path, 
    original_path, 
    ext, 
    mime_type, 
    linked_entity_type, 
    linked_entity_id, 
    linked_entity_meta, 
    created_at, 
    updated_at
)
SELECT 
    m.id,
    COALESCE(NULLIF(m.path, ''), m.original_path) as title, -- Use path as title, fallback to original_path
    '' as description, -- Empty description
    'document' as type,
    '[]'::jsonb as tags, -- Empty tags array
    m.path,
    m.original_path,
    m.ext,
    m.mime_type,
    'commodity' as linked_entity_type,
    m.commodity_id as linked_entity_id,
    'manuals' as linked_entity_meta,
    NOW() as created_at,
    NOW() as updated_at
FROM manuals m
WHERE NOT EXISTS (
    SELECT 1 FROM files f 
    WHERE f.id = m.id
);

-- Migrate invoices to file entities
INSERT INTO files (
    id, 
    title, 
    description, 
    type, 
    tags, 
    path, 
    original_path, 
    ext, 
    mime_type, 
    linked_entity_type, 
    linked_entity_id, 
    linked_entity_meta, 
    created_at, 
    updated_at
)
SELECT 
    inv.id,
    COALESCE(NULLIF(inv.path, ''), inv.original_path) as title, -- Use path as title, fallback to original_path
    '' as description, -- Empty description
    'document' as type,
    '[]'::jsonb as tags, -- Empty tags array
    inv.path,
    inv.original_path,
    inv.ext,
    inv.mime_type,
    'commodity' as linked_entity_type,
    inv.commodity_id as linked_entity_id,
    'invoices' as linked_entity_meta,
    NOW() as created_at,
    NOW() as updated_at
FROM invoices inv
WHERE NOT EXISTS (
    SELECT 1 FROM files f 
    WHERE f.id = inv.id
);
