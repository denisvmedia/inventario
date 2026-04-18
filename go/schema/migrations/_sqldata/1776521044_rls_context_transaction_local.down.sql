-- Restore the original pre-hardening definitions of the three RLS setters
-- (session-scoped set_config + SECURITY DEFINER).

CREATE OR REPLACE FUNCTION set_tenant_context(tenant_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION set_user_context(user_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_user_id', user_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION set_group_context(group_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_group_id', group_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;
