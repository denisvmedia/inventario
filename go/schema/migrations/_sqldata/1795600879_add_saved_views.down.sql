-- Migration rollback
-- Generated on: 2026-11-25T11:01:19+01:00
-- Direction: DOWN
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

DROP INDEX IF EXISTS "idx_saved_view_placements_group_user";
DROP INDEX IF EXISTS "idx_saved_view_placements_user";
DROP INDEX IF EXISTS "idx_saved_view_placements_uuid";
DROP INDEX IF EXISTS "idx_saved_view_placements_view_user";
DROP INDEX IF EXISTS "idx_saved_views_created_by";
DROP INDEX IF EXISTS "idx_saved_views_tenant_group_order";
DROP INDEX IF EXISTS "idx_saved_views_tenant_id";
DROP INDEX IF EXISTS "idx_saved_views_uuid";
-- Drop RLS policy saved_view_placement_background_worker_access from table saved_view_placements
DROP POLICY IF EXISTS "saved_view_placement_background_worker_access" ON "saved_view_placements";
-- Drop RLS policy saved_view_placement_isolation from table saved_view_placements
DROP POLICY IF EXISTS "saved_view_placement_isolation" ON "saved_view_placements";
-- Drop RLS policy saved_view_background_worker_access from table saved_views
DROP POLICY IF EXISTS "saved_view_background_worker_access" ON "saved_views";
-- Drop RLS policy saved_view_isolation from table saved_views
DROP POLICY IF EXISTS "saved_view_isolation" ON "saved_views";
-- ALTER statements: --
ALTER TABLE "saved_view_placements" DROP CONSTRAINT IF EXISTS "fk_saved_view_placement_group";
-- ALTER statements: --
ALTER TABLE "saved_view_placements" DROP CONSTRAINT IF EXISTS "fk_saved_view_placement_view";
-- ALTER statements: --
ALTER TABLE "saved_view_placements" DROP CONSTRAINT IF EXISTS "fk_saved_view_placement_user";
-- ALTER statements: --
ALTER TABLE "saved_views" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- ALTER statements: --
ALTER TABLE "saved_views" DROP CONSTRAINT IF EXISTS "fk_entity_group";
-- ALTER statements: --
ALTER TABLE "saved_views" DROP CONSTRAINT IF EXISTS "fk_entity_created_by";
-- ALTER statements: --
ALTER TABLE "saved_view_placements" DROP CONSTRAINT IF EXISTS "fk_entity_tenant";
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "saved_view_placements" CASCADE;
-- WARNING: This will delete all data!
DROP TABLE IF EXISTS "saved_views" CASCADE;