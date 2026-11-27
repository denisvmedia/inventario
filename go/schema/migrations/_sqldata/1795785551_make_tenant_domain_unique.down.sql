-- Migration rollback
-- Generated on: 2026-11-27T14:19:11+01:00
-- Direction: DOWN

CREATE INDEX "tenants_domain_idx" ON "tenants" ("domain");
DROP INDEX "tenants_domain_unique_idx";