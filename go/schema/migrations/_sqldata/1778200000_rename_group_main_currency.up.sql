-- Rename main_currency → group_currency on location_groups (#1472).
-- Hand-written rather than schema-diff generated because the migrator
-- can't tell a rename from a drop+add.
ALTER TABLE location_groups RENAME COLUMN main_currency TO group_currency;
