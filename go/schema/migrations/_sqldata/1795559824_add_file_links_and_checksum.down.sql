-- Migration rollback
-- Generated on: 2026-11-24T23:37:04+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_file_links_entity";
DROP INDEX IF EXISTS "idx_file_links_file_entity";
DROP INDEX IF EXISTS "idx_file_links_tenant_group";
DROP INDEX IF EXISTS "idx_file_links_uuid";
DROP INDEX IF EXISTS "idx_files_tenant_group_sha256";
-- Drop RLS policy file_link_background_worker_access from table file_links
DROP POLICY IF EXISTS "file_link_background_worker_access" ON "file_links";
-- Drop RLS policy file_link_isolation from table file_links
DROP POLICY IF EXISTS "file_link_isolation" ON "file_links";
-- Remove columns from table: files --
-- ALTER statements: --
ALTER TABLE "files" DROP COLUMN "sha256" CASCADE;
-- WARNING: Dropping column files.sha256 with CASCADE - This will delete data and dependent objects! --
-- ALTER statements: --
ALTER TABLE "file_links" DROP CONSTRAINT IF EXISTS "fk_file_link_file";
-- ALTER statements: --
ALTER TABLE "file_links" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "file_links" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "file_links" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "file_links" CASCADE;