-- Migration: add a free-form `description` column to location_groups
-- so the group settings page (#1537 item 4) and the sidebar group switcher
-- (#1537 item 5) can render a one-line muted subtitle per group.
-- Direction: UP
--
-- Mirrors the locations.description pattern from 1779500000 (issue #1531
-- item 4): TEXT NOT NULL DEFAULT '' so existing rows keep working with an
-- empty subtitle until the admin sets one. Issue #1647.

ALTER TABLE location_groups ADD COLUMN description TEXT NOT NULL DEFAULT '';
