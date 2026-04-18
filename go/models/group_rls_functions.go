package models

// Group RLS Functions for Group-Level Database Isolation
//
// These functions provide session-based group context management
// for Row-Level Security policies to work effectively alongside
// the existing tenant-based and user-based isolation.

// Group context functions for Row-Level Security.
//
// The setter scopes the GUC to the current transaction (`set_config(..., true)`
// == `SET LOCAL`) so a pooled DB connection cannot leak `app.current_group_id`
// into the next request's transaction. SECURITY DEFINER is deliberately NOT
// set — set_config needs no elevated privilege, and DEFINER would unnecessarily
// widen the blast radius if someone were to chain a function call into it.
// The getter is STABLE and reads with the missing_ok=true flag so callers
// before a group has been set get an empty string instead of an error.
type GroupRLSFunctions struct {
	// Function to set the current group context in the session
	//migrator:schema:function name="set_group_context" params="group_id_param TEXT" returns="VOID" language="plpgsql" body="BEGIN PERFORM set_config('app.current_group_id', group_id_param, true); END;" comment="Sets the current group context for RLS policies (transaction-local)"
	_ int

	// Function to get the current group ID from the session
	//migrator:schema:function name="get_current_group_id" returns="TEXT" language="plpgsql" volatility="STABLE" body="BEGIN RETURN current_setting('app.current_group_id', true); END;" comment="Gets the current group ID from session for RLS policies"
	_ int
}
