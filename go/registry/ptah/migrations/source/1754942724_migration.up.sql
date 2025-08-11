-- Migration generated from schema differences
-- Generated on: 2025-08-11T20:05:24Z
-- Direction: UP

-- Gets the current user ID from session for RLS policies
CREATE OR REPLACE FUNCTION get_current_user_id() RETURNS TEXT AS $$
BEGIN RETURN current_setting('app.current_user_id', true); END;
$$
LANGUAGE plpgsql STABLE;
-- Sets the current user context for RLS policies
CREATE OR REPLACE FUNCTION set_user_context(user_id_param TEXT) RETURNS VOID AS $$
BEGIN PERFORM set_config('app.current_user_id', user_id_param, false); END;
$$
LANGUAGE plpgsql SECURITY DEFINER;