-- Migration rollback
-- Generated on: 2026-11-24T23:40:09+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "commodities_search_text_trgm_idx";
-- Remove columns from table: commodities --
-- ALTER statements: --
ALTER TABLE "commodities" DROP COLUMN "search_text" CASCADE;
-- WARNING: Dropping column commodities.search_text with CASCADE - This will delete data and dependent objects! --
-- WARNING: Ensure no other objects depend on this function
DROP FUNCTION IF EXISTS "commodity_search_text"(name text, short_name text, serial_number text, extra_serial_numbers jsonb, part_numbers jsonb, tags jsonb, comments text, warranty_notes text, status_note text);