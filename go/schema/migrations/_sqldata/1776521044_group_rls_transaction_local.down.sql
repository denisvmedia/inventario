-- Restore the original Phase 1 definition of set_group_context
-- (session-scoped set_config + SECURITY DEFINER).

CREATE OR REPLACE FUNCTION set_group_context(group_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_group_id', group_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;
