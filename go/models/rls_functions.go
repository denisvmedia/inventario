package models

// RLS Functions for Multi-Tenant Database-Level Isolation
//
// These functions provide session-based tenant context management
// for Row-Level Security policies to work effectively.

// Database roles for multi-tenant access control
type DatabaseRoles struct {
	// Application role for RLS policies
	//xmigrator:schema:role name="inventario_app" login="false" comment="Application role for Row-Level Security policies"
	_ int
}

// Multi-tenant context functions for Row-Level Security.
//
// The setter scopes the GUC to the current transaction
// (`set_config(..., true)` == `SET LOCAL`) so a pgbouncer-pooled
// connection can't leak `app.current_tenant_id` into the next
// request's transaction. SECURITY DEFINER is deliberately NOT set —
// set_config requires no elevated privilege. Matches
// set_group_context / set_user_context.
type RLSFunctions struct {
	// Function to set the current tenant context in the session
	//migrator:schema:function name="set_tenant_context" params="tenant_id_param TEXT" returns="VOID" language="plpgsql" body="BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, true); END;" comment="Sets the current tenant context for RLS policies (transaction-local)"
	_ int

	// Function to get the current tenant ID from the session
	//migrator:schema:function name="get_current_tenant_id" returns="TEXT" language="plpgsql" volatility="STABLE" body="BEGIN RETURN current_setting('app.current_tenant_id', true); END;" comment="Gets the current tenant ID from session for RLS policies"
	_ int
}
