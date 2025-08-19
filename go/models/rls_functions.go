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

// Multi-tenant context functions for Row-Level Security
type RLSFunctions struct {
	// Function to set the current tenant context in the session
	//migrator:schema:function name="set_tenant_context" params="tenant_id_param TEXT" returns="VOID" language="plpgsql" security="DEFINER" body="BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, false); END;" comment="Sets the current tenant context for RLS policies"
	_ int

	// Function to get the current tenant ID from the session
	//migrator:schema:function name="get_current_tenant_id" returns="TEXT" language="plpgsql" volatility="STABLE" body="BEGIN RETURN current_setting('app.current_tenant_id', true); END;" comment="Gets the current tenant ID from session for RLS policies"
	_ int
}
