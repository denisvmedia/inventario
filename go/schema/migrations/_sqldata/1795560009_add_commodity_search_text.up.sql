-- Migration generated from schema differences
-- Generated on: 2026-11-24T23:40:09+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- Builds commodities.search_text, the text the item search matches
CREATE OR REPLACE FUNCTION "commodity_search_text"(name text, short_name text, serial_number text, extra_serial_numbers jsonb, part_numbers jsonb, tags jsonb, comments text, warranty_notes text, status_note text) RETURNS text AS $$
SELECT COALESCE(name, '') || pg_catalog.chr(31) || COALESCE(short_name, '') || pg_catalog.chr(31) || COALESCE(serial_number, '') || pg_catalog.chr(31) || COALESCE((SELECT pg_catalog.string_agg(e, pg_catalog.chr(31)) FROM pg_catalog.jsonb_array_elements_text(CASE WHEN pg_catalog.jsonb_typeof(extra_serial_numbers) = 'array' THEN extra_serial_numbers END) AS e), '') || pg_catalog.chr(31) || COALESCE((SELECT pg_catalog.string_agg(e, pg_catalog.chr(31)) FROM pg_catalog.jsonb_array_elements_text(CASE WHEN pg_catalog.jsonb_typeof(part_numbers) = 'array' THEN part_numbers END) AS e), '') || pg_catalog.chr(31) || COALESCE((SELECT pg_catalog.string_agg(e, pg_catalog.chr(31)) FROM pg_catalog.jsonb_array_elements_text(CASE WHEN pg_catalog.jsonb_typeof(tags) = 'array' THEN tags END) AS e), '') || pg_catalog.chr(31) || COALESCE(comments, '') || pg_catalog.chr(31) || COALESCE(warranty_notes, '') || pg_catalog.chr(31) || COALESCE(status_note, '')
$$
LANGUAGE sql SECURITY INVOKER IMMUTABLE;
-- Add/modify columns for table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" ADD COLUMN "search_text" TEXT GENERATED ALWAYS AS (commodity_search_text(name, short_name, serial_number, extra_serial_numbers, part_numbers, tags, comments, warranty_notes, status_note)) STORED;
CREATE INDEX IF NOT EXISTS "commodities_search_text_trgm_idx" ON "commodities" USING GIN ("search_text" gin_trgm_ops);