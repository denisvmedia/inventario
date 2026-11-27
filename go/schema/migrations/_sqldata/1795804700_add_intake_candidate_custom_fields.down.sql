-- Migration rollback
-- Generated on: 2026-11-27T19:38:21+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "intake_candidates" DROP COLUMN "custom_fields";
ALTER TABLE "intake_candidates" DROP COLUMN "accepted_custom_fields";