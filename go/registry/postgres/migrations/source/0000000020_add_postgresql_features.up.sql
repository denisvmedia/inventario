-- Add PostgreSQL-specific advanced features
-- This migration adds full-text search, advanced indexing, and other PostgreSQL-specific capabilities

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_trgm;  -- For similarity search
CREATE EXTENSION IF NOT EXISTS btree_gin; -- For composite indexes

-- Add full-text search capabilities to commodities
ALTER TABLE commodities ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Create function to update search vector
CREATE OR REPLACE FUNCTION update_commodity_search_vector() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := 
        setweight(to_tsvector('english', COALESCE(NEW.name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.short_name, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.comments, '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(NEW.serial_number, '')), 'D') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.tags, ' '), '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to maintain search vector
DROP TRIGGER IF EXISTS commodity_search_vector_update ON commodities;
CREATE TRIGGER commodity_search_vector_update 
    BEFORE INSERT OR UPDATE ON commodities
    FOR EACH ROW EXECUTE FUNCTION update_commodity_search_vector();

-- Create advanced indexes for commodities
CREATE INDEX IF NOT EXISTS commodities_search_vector_idx ON commodities USING GIN (search_vector);
CREATE INDEX IF NOT EXISTS commodities_tags_gin_idx ON commodities USING GIN (tags);
CREATE INDEX IF NOT EXISTS commodities_extra_serial_numbers_gin_idx ON commodities USING GIN (extra_serial_numbers);
CREATE INDEX IF NOT EXISTS commodities_part_numbers_gin_idx ON commodities USING GIN (part_numbers);
CREATE INDEX IF NOT EXISTS commodities_urls_gin_idx ON commodities USING GIN (urls);

-- Create partial indexes for common queries
CREATE INDEX IF NOT EXISTS commodities_active_idx ON commodities (status, area_id) WHERE draft = false;
CREATE INDEX IF NOT EXISTS commodities_draft_idx ON commodities (last_modified_date) WHERE draft = true;
CREATE INDEX IF NOT EXISTS commodities_price_idx ON commodities (converted_original_price) WHERE converted_original_price IS NOT NULL;
CREATE INDEX IF NOT EXISTS commodities_purchase_date_idx ON commodities (purchase_date) WHERE purchase_date IS NOT NULL;

-- Create trigram indexes for similarity search
CREATE INDEX IF NOT EXISTS commodities_name_trgm_idx ON commodities USING GIN (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS commodities_short_name_trgm_idx ON commodities USING GIN (short_name gin_trgm_ops);

-- Add full-text search capabilities to files
ALTER TABLE files ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Create function to update file search vector
CREATE OR REPLACE FUNCTION update_file_search_vector() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := 
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.path, '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(NEW.original_path, '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.tags, ' '), '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to maintain file search vector
DROP TRIGGER IF EXISTS file_search_vector_update ON files;
CREATE TRIGGER file_search_vector_update 
    BEFORE INSERT OR UPDATE ON files
    FOR EACH ROW EXECUTE FUNCTION update_file_search_vector();

-- Create advanced indexes for files
CREATE INDEX IF NOT EXISTS files_search_vector_idx ON files USING GIN (search_vector);
CREATE INDEX IF NOT EXISTS files_tags_gin_idx ON files USING GIN (tags);
CREATE INDEX IF NOT EXISTS files_type_created_idx ON files (type, created_at);
CREATE INDEX IF NOT EXISTS files_linked_entity_idx ON files (linked_entity_type, linked_entity_id);
CREATE INDEX IF NOT EXISTS files_linked_entity_meta_idx ON files (linked_entity_type, linked_entity_id, linked_entity_meta);

-- Create trigram indexes for file similarity search
CREATE INDEX IF NOT EXISTS files_title_trgm_idx ON files USING GIN (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS files_path_trgm_idx ON files USING GIN (path gin_trgm_ops);

-- Add indexes for areas
CREATE INDEX IF NOT EXISTS areas_name_trgm_idx ON areas USING GIN (name gin_trgm_ops);

-- Add indexes for locations
CREATE INDEX IF NOT EXISTS locations_name_trgm_idx ON locations USING GIN (name gin_trgm_ops);

-- Create composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS commodities_area_status_idx ON commodities (area_id, status) WHERE draft = false;
CREATE INDEX IF NOT EXISTS commodities_type_status_idx ON commodities (type, status) WHERE draft = false;
CREATE INDEX IF NOT EXISTS files_type_entity_idx ON files (type, linked_entity_type, linked_entity_id);

-- Add indexes for export operations
CREATE INDEX IF NOT EXISTS exports_status_created_idx ON exports (status, created_date);
CREATE INDEX IF NOT EXISTS exports_type_status_idx ON exports (type, status);

-- Update existing records to populate search vectors
UPDATE commodities SET search_vector = 
    setweight(to_tsvector('english', COALESCE(name, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(short_name, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(comments, '')), 'C') ||
    setweight(to_tsvector('english', COALESCE(serial_number, '')), 'D') ||
    setweight(to_tsvector('english', COALESCE(array_to_string(tags, ' '), '')), 'D')
WHERE search_vector IS NULL;

UPDATE files SET search_vector = 
    setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('english', COALESCE(path, '')), 'C') ||
    setweight(to_tsvector('english', COALESCE(original_path, '')), 'C') ||
    setweight(to_tsvector('english', COALESCE(array_to_string(tags, ' '), '')), 'D')
WHERE search_vector IS NULL;

-- Create materialized view for commodity statistics (optional optimization)
CREATE MATERIALIZED VIEW IF NOT EXISTS commodity_stats AS
SELECT 
    area_id,
    COUNT(*) as total_count,
    COUNT(*) FILTER (WHERE draft = false) as active_count,
    COUNT(*) FILTER (WHERE draft = true) as draft_count,
    AVG(COALESCE(converted_original_price, original_price)) as avg_price,
    SUM(COALESCE(converted_original_price, original_price)) as total_value,
    COUNT(DISTINCT type) as type_count,
    array_agg(DISTINCT status) as statuses
FROM commodities 
GROUP BY area_id;

-- Create unique index on materialized view
CREATE UNIQUE INDEX IF NOT EXISTS commodity_stats_area_idx ON commodity_stats (area_id);

-- Create function to refresh commodity stats
CREATE OR REPLACE FUNCTION refresh_commodity_stats() RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY commodity_stats;
END;
$$ LANGUAGE plpgsql;

-- Add comments for documentation
COMMENT ON COLUMN commodities.search_vector IS 'Full-text search vector for commodity content';
COMMENT ON COLUMN files.search_vector IS 'Full-text search vector for file content';
COMMENT ON FUNCTION update_commodity_search_vector() IS 'Trigger function to maintain commodity search vector';
COMMENT ON FUNCTION update_file_search_vector() IS 'Trigger function to maintain file search vector';
COMMENT ON MATERIALIZED VIEW commodity_stats IS 'Aggregated statistics for commodities by area';
COMMENT ON FUNCTION refresh_commodity_stats() IS 'Refresh the commodity statistics materialized view';
