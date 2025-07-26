-- Rollback migration of commodity files from file entities back to separate tables

-- Remove file entities that were migrated from images
DELETE FROM files 
WHERE linked_entity_type = 'commodity' 
  AND linked_entity_meta = 'images'
  AND id IN (SELECT id FROM images);

-- Remove file entities that were migrated from manuals  
DELETE FROM files 
WHERE linked_entity_type = 'commodity' 
  AND linked_entity_meta = 'manuals'
  AND id IN (SELECT id FROM manuals);

-- Remove file entities that were migrated from invoices
DELETE FROM files 
WHERE linked_entity_type = 'commodity' 
  AND linked_entity_meta = 'invoices'
  AND id IN (SELECT id FROM invoices);
