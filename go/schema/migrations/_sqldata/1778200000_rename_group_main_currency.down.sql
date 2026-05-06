-- Reverse rename group_currency → main_currency on location_groups (#1472).
ALTER TABLE location_groups RENAME COLUMN group_currency TO main_currency;
