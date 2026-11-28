-- Migration rollback
-- Generated on: 2026-11-28T09:51:44+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "files" ALTER COLUMN "linked_entity_id" DROP DEFAULT;
ALTER TABLE "files" ALTER COLUMN "linked_entity_meta" DROP DEFAULT;
ALTER TABLE "files" ALTER COLUMN "linked_entity_type" DROP DEFAULT;