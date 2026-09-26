-- Migration generated from schema differences
-- Generated on: 2026-09-26T11:58:15+02:00
-- Direction: UP
-- +ptah lock_timeout=3s
-- +ptah statement_timeout=30s

-- POSTGRES TABLE: weekly_digest_sends --
CREATE TABLE "weekly_digest_sends" (
  "week_start" DATE NOT NULL,
  "sent_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "tenant_id" TEXT NOT NULL,
  "user_id" TEXT NOT NULL,
  "id" TEXT PRIMARY KEY NOT NULL,
  "uuid" TEXT NOT NULL DEFAULT (gen_random_uuid())::text
);
-- Enable RLS for weekly_digest_sends table
ALTER TABLE "weekly_digest_sends" ENABLE ROW LEVEL SECURITY;
-- Allows the digest worker to record sends across every tenant, where no user context exists
DROP POLICY IF EXISTS "weekly_digest_send_background_worker_access" ON "weekly_digest_sends";
CREATE POLICY "weekly_digest_send_background_worker_access" ON "weekly_digest_sends" FOR ALL TO "inventario_background_worker"
    USING (true)
    WITH CHECK (true);
-- Ensures a digest send record is visible only to the user it was sent to, within their tenant
DROP POLICY IF EXISTS "weekly_digest_send_isolation" ON "weekly_digest_sends";
CREATE POLICY "weekly_digest_send_isolation" ON "weekly_digest_sends" FOR ALL TO "inventario_app"
    USING (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '')
    WITH CHECK (tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != '');
CREATE UNIQUE INDEX IF NOT EXISTS "idx_weekly_digest_sends_user_week" ON "weekly_digest_sends" ("user_id", "week_start");
CREATE INDEX IF NOT EXISTS "idx_weekly_digest_sends_week_start" ON "weekly_digest_sends" ("week_start");
-- ALTER statements: --
ALTER TABLE "weekly_digest_sends" ADD CONSTRAINT "fk_entity_tenant" FOREIGN KEY ("tenant_id") REFERENCES "tenants"("id");
-- ALTER statements: --
ALTER TABLE "weekly_digest_sends" ADD CONSTRAINT "fk_entity_user" FOREIGN KEY ("user_id") REFERENCES "users"("id");