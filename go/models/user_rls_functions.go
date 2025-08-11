package models

// User RLS Functions for User-Level Database Isolation
//
// These functions provide session-based user context management
// for Row-Level Security policies to work effectively alongside
// the existing tenant-based isolation.

// User context functions for Row-Level Security
type UserRLSFunctions struct {
	// Function to set the current user context in the session
	//migrator:schema:function name="set_user_context" params="user_id_param TEXT" returns="VOID" language="plpgsql" security="DEFINER" body="BEGIN PERFORM set_config('app.current_user_id', user_id_param, false); END;" comment="Sets the current user context for RLS policies"
	_ int

	// Function to get the current user ID from the session
	//migrator:schema:function name="get_current_user_id" returns="TEXT" language="plpgsql" volatility="STABLE" body="BEGIN RETURN current_setting('app.current_user_id', true); END;" comment="Gets the current user ID from session for RLS policies"
	_ int
}
