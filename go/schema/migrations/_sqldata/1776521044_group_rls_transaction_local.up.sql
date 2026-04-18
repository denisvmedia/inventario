-- Replace set_group_context to use a transaction-local GUC instead of a
-- session-scoped one, and drop the unnecessary SECURITY DEFINER.
--
-- The original Phase 1 definition used `set_config(..., false)` which writes
-- the GUC at session scope — fine for a dedicated per-request connection,
-- but pgx/pgbouncer-pooled connections are reused across requests, and a
-- session-scoped `app.current_group_id` from one request can survive into
-- the next if the caller forgets to explicitly reset it. `true` scopes it
-- to the current transaction, so the value disappears at COMMIT/ROLLBACK
-- without requiring any cleanup code.
--
-- SECURITY DEFINER is also dropped — set_config needs no elevated privilege,
-- and DEFINER would unnecessarily widen the blast radius if someone chained
-- a function call into it.
--
-- Ptah cannot detect function-body changes (it compares signatures only),
-- so this migration is hand-written and not regenerable via
-- ./scripts/generate-migration.sh. Matches the set_tenant_context /
-- set_user_context definitions in the earlier multitenant migration,
-- which should be updated in the same way in a follow-up.

CREATE OR REPLACE FUNCTION set_group_context(group_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_group_id', group_id_param, true); END;
$$
LANGUAGE plpgsql;
