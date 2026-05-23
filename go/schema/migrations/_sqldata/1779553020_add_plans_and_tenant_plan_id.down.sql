-- Migration rollback: drop tenants.plan_id (issue #1389).
-- Direction: DOWN

DROP INDEX IF EXISTS idx_tenants_plan_id;
ALTER TABLE tenants DROP COLUMN plan_id CASCADE;
