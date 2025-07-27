-- Migration rollback
-- Generated on: 2025-07-27T17:17:21+02:00
-- Direction: DOWN

DROP INDEX IF EXISTS commodities_active_idx;
DROP INDEX IF EXISTS commodities_draft_idx;
DROP INDEX IF EXISTS commodities_extra_serial_numbers_gin_idx;
DROP INDEX IF EXISTS commodities_name_trgm_idx;
DROP INDEX IF EXISTS commodities_part_numbers_gin_idx;
DROP INDEX IF EXISTS commodities_short_name_trgm_idx;
DROP INDEX IF EXISTS commodities_tags_gin_idx;
DROP INDEX IF EXISTS commodities_urls_gin_idx;
DROP INDEX IF EXISTS files_linked_entity_idx;
DROP INDEX IF EXISTS files_linked_entity_meta_idx;
DROP INDEX IF EXISTS files_path_trgm_idx;
DROP INDEX IF EXISTS files_tags_gin_idx;
DROP INDEX IF EXISTS files_title_trgm_idx;
DROP INDEX IF EXISTS files_type_created_idx;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS areas CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS commodities CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS exports CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS files CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS images CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS invoices CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS locations CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS manuals CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS restore_operations CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS restore_steps CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS settings CASCADE;
-- WARNING: Removing extension 'btree_gin' may break existing functionality that depends on it --
-- Consider reviewing all database objects that use this extension before proceeding --
-- Extension removal may cascade to dependent objects - review carefully --
-- Remove extension 'btree_gin' as it's no longer required by the schema
DROP EXTENSION IF EXISTS btree_gin;
--  --
-- WARNING: Removing extension 'pg_trgm' may break existing functionality that depends on it --
-- Consider reviewing all database objects that use this extension before proceeding --
-- Extension removal may cascade to dependent objects - review carefully --
-- Remove extension 'pg_trgm' as it's no longer required by the schema
DROP EXTENSION IF EXISTS pg_trgm;