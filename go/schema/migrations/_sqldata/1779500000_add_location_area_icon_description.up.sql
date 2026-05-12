-- Migration: add `icon` + `description` columns to locations and `icon`
-- column to areas (issue #1531 item 4).
-- Direction: UP
--
-- Locations carry an emoji avatar + a one-line muted description per the
-- design mock (`design-mocks/src/views/LocationPickerView.tsx` L546-L600).
-- Areas carry an emoji avatar only (mock L615-L668). All three columns
-- default to empty string so existing rows continue to render via the
-- generic Lucide MapPin / Package fallbacks until the user picks values.

ALTER TABLE locations ADD COLUMN icon TEXT NOT NULL DEFAULT '';
ALTER TABLE locations ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE areas ADD COLUMN icon TEXT NOT NULL DEFAULT '';
