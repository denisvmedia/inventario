package models

// Group RLS Functions for Group-Level Database Isolation
//
// These functions provide session-based group context management
// for Row-Level Security policies to work effectively alongside
// the existing tenant-based and user-based isolation.

// Group context functions for Row-Level Security
type GroupRLSFunctions struct {
	// Function to set the current group context in the session
	//migrator:schema:function name="set_group_context" params="group_id_param TEXT" returns="VOID" language="plpgsql" security="DEFINER" body="BEGIN PERFORM set_config('app.current_group_id', group_id_param, false); END;" comment="Sets the current group context for RLS policies"
	_ int

	// Function to get the current group ID from the session
	//migrator:schema:function name="get_current_group_id" returns="TEXT" language="plpgsql" volatility="STABLE" body="BEGIN RETURN current_setting('app.current_group_id', true); END;" comment="Gets the current group ID from session for RLS policies"
	_ int
}
