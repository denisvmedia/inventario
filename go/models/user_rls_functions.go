package models

// User RLS Functions for User-Level Database Isolation
//
// These functions provide session-based user context management
// for Row-Level Security policies to work effectively alongside
// the existing tenant-based isolation.

// UserRLSFunctions carries the user context functions for Row-Level Security.
//
// The setter scopes the GUC to the current transaction
// (`set_config(..., true)` == `SET LOCAL`) so a pgbouncer-pooled
// connection can't leak `app.current_user_id` into the next request's
// transaction. SECURITY DEFINER is deliberately NOT set —
// set_config requires no elevated privilege. Matches
// set_tenant_context / set_group_context.
//
// The annotations sit on the type's doc comment, not on struct fields: Ptah
// collects `ptah:schema:function` from the declaration a comment is attached
// to, and a field of a struct that declares no table is never reached.
//
//ptah:schema:function name="set_user_context" params="user_id_param TEXT" returns="VOID" language="plpgsql" body="BEGIN PERFORM set_config('app.current_user_id', user_id_param, true); END;" comment="Sets the current user context for RLS policies (transaction-local)"
//ptah:schema:function name="get_current_user_id" returns="TEXT" language="plpgsql" volatility="STABLE" body="BEGIN RETURN current_setting('app.current_user_id', true); END;" comment="Gets the current user ID from session for RLS policies"
type UserRLSFunctions struct {
	_ int
}
