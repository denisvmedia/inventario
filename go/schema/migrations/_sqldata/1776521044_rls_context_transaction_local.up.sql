-- Harden the three RLS setter helpers:
--   set_tenant_context / set_user_context / set_group_context
--
-- Two changes per function:
--   1. Switch the GUC scope from session (`set_config(..., false)`) to
--      transaction-local (`set_config(..., true)` == `SET LOCAL`). A
--      pgbouncer-pooled connection otherwise carries the GUC across
--      requests, which in the worst case leaks `app.current_tenant_id` /
--      `app.current_user_id` / `app.current_group_id` into a request for
--      a different principal. Transaction scope clears it at COMMIT /
--      ROLLBACK automatically.
--   2. Drop `SECURITY DEFINER`. set_config requires no elevated privilege,
--      and DEFINER unnecessarily widens the blast radius.
--
-- Ptah cannot detect function-body / security-attribute changes (tracked
-- upstream in stokaro/ptah#89), so this migration is hand-written and not
-- regenerable via ./scripts/generate-migration.sh. The Go annotations in
-- models/rls_functions.go, models/user_rls_functions.go, and
-- models/group_rls_functions.go were updated in the same commit so a
-- fresh install of the migrations reaches the same final state.

CREATE OR REPLACE FUNCTION set_tenant_context(tenant_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, true); END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION set_user_context(user_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_user_id', user_id_param, true); END;
$$
LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION set_group_context(group_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_group_id', group_id_param, true); END;
$$
LANGUAGE plpgsql;
