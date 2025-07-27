-- Remove PostgreSQL-specific advanced features
-- This migration removes full-text search, advanced indexing, and other PostgreSQL-specific capabilities

-- Drop materialized view and related functions
DROP FUNCTION IF EXISTS refresh_commodity_stats();
DROP MATERIALIZED VIEW IF EXISTS commodity_stats;

-- Drop triggers
DROP TRIGGER IF EXISTS commodity_search_vector_update ON commodities;
DROP TRIGGER IF EXISTS file_search_vector_update ON files;

-- Drop trigger functions
DROP FUNCTION IF EXISTS update_commodity_search_vector();
DROP FUNCTION IF EXISTS update_file_search_vector();

-- Drop advanced indexes for commodities
DROP INDEX IF EXISTS commodities_search_vector_idx;
DROP INDEX IF EXISTS commodities_tags_gin_idx;
DROP INDEX IF EXISTS commodities_extra_serial_numbers_gin_idx;
DROP INDEX IF EXISTS commodities_part_numbers_gin_idx;
DROP INDEX IF EXISTS commodities_urls_gin_idx;
DROP INDEX IF EXISTS commodities_active_idx;
DROP INDEX IF EXISTS commodities_draft_idx;
DROP INDEX IF EXISTS commodities_price_idx;
DROP INDEX IF EXISTS commodities_purchase_date_idx;
DROP INDEX IF EXISTS commodities_name_trgm_idx;
DROP INDEX IF EXISTS commodities_short_name_trgm_idx;

-- Drop advanced indexes for files
DROP INDEX IF EXISTS files_search_vector_idx;
DROP INDEX IF EXISTS files_tags_gin_idx;
DROP INDEX IF EXISTS files_type_created_idx;
DROP INDEX IF EXISTS files_linked_entity_idx;
DROP INDEX IF EXISTS files_linked_entity_meta_idx;
DROP INDEX IF EXISTS files_title_trgm_idx;
DROP INDEX IF EXISTS files_path_trgm_idx;

-- Drop indexes for areas and locations
DROP INDEX IF EXISTS areas_name_trgm_idx;
DROP INDEX IF EXISTS locations_name_trgm_idx;

-- Drop composite indexes
DROP INDEX IF EXISTS commodities_area_status_idx;
DROP INDEX IF EXISTS commodities_type_status_idx;
DROP INDEX IF EXISTS files_type_entity_idx;

-- Drop export indexes
DROP INDEX IF EXISTS exports_status_created_idx;
DROP INDEX IF EXISTS exports_type_status_idx;

-- Remove search vector columns
ALTER TABLE commodities DROP COLUMN IF EXISTS search_vector;
ALTER TABLE files DROP COLUMN IF EXISTS search_vector;

-- Note: We don't drop extensions as they might be used by other applications
-- DROP EXTENSION IF EXISTS pg_trgm;
-- DROP EXTENSION IF EXISTS btree_gin;
