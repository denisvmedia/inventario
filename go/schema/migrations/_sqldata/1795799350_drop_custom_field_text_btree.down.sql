-- Migration rollback
-- Generated on: 2026-11-27T18:09:10+01:00
-- Direction: DOWN

CREATE INDEX "idx_commodity_custom_field_values_field_text" ON "commodity_custom_field_values" ("field_id", "value_text");