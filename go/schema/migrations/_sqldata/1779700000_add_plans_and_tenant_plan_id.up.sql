-- Migration: link tenants to a subscription plan via `tenants.plan_id`
-- (issue #1389 — minimum slice that unblocks the Plan & quota card from
-- #1537 item 1).
-- Direction: UP
--
-- Scope: tenant.plan_id only. Plan definitions themselves are kept in
-- Go code (`go/models/plan.go`) for v1 — there is no need for a
-- separate `plans` table until we add operator override (admin UI to
-- edit plan limits per-tenant) or billing integration, both of which
-- are out of scope for this iteration. `plan_id` is plain TEXT (no FK)
-- so the column moves no faster than the in-code allowlist; the model
-- validates the value at Tenant.Validate time, and an unknown id at
-- read time degrades to the `unlimited` plan (safer than failing the
-- request — see `models.PlanByID`).
--
-- Self-hosters get `unlimited` as the default so single-user installs
-- feel like nothing changed when this lands (AC of #1389 calls this
-- out explicitly).

ALTER TABLE tenants ADD COLUMN plan_id TEXT NOT NULL DEFAULT 'unlimited';

CREATE INDEX idx_tenants_plan_id ON tenants(plan_id);
