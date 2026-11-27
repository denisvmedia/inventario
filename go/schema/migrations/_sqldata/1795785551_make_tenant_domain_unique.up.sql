-- Migration generated from schema differences
-- Generated on: 2026-11-27T14:19:11+01:00
-- Direction: UP

CREATE UNIQUE INDEX "tenants_domain_unique_idx" ON "tenants" ("domain") WHERE domain IS NOT NULL;
DROP INDEX "tenants_domain_idx";