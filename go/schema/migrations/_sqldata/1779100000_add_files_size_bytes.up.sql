-- Add file blob size for per-group storage-usage aggregation (#1388).
-- Existing rows default to 0 and are backfilled on first server boot by
-- StorageUsageBackfill walking the configured upload bucket. NOT NULL so
-- the SUM() aggregator never has to handle NULL.

ALTER TABLE files ADD COLUMN size_bytes BIGINT NOT NULL DEFAULT 0;
