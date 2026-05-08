-- Reverse #1388 size_bytes column on files.
-- Storage-usage endpoint depends on this column; rolling back disables
-- the breakdown but leaves all blobs intact.

ALTER TABLE files DROP COLUMN size_bytes CASCADE;
