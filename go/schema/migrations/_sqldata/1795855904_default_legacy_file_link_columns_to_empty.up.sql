-- Migration generated from schema differences
-- Generated on: 2026-11-28T09:51:44+01:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

ALTER TABLE "files" ALTER COLUMN "linked_entity_id" SET DEFAULT '';
ALTER TABLE "files" ALTER COLUMN "linked_entity_meta" SET DEFAULT '';
ALTER TABLE "files" ALTER COLUMN "linked_entity_type" SET DEFAULT '';