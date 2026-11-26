package postgres

import "go.5x5.cz/inventario/registry/postgres/store"

// UserPurgeTableNames exposes the tables UserPurger deletes by user_id to the
// black-box test package, so a test can check that list against the live schema
// rather than restating it. It compiles only under `go test`.
func UserPurgeTableNames() []string {
	names := make([]string, 0, len(userDeleteByUserID))
	for _, nameFn := range userDeleteByUserID {
		names = append(names, nameFn(store.DefaultTableNames))
	}
	return names
}

// TenantPurgeTableNames exposes every table the tenant purge deletes from by
// tenant_id: the FK-ordered list, then audit_logs, location_groups and users,
// which PurgeTenantDependents handles after it. A test checks the list against
// the live schema. It compiles only under `go test`.
func TenantPurgeTableNames() []string {
	names := make([]string, 0, len(tenantPurgeOrder)+3)
	for _, nameFn := range tenantPurgeOrder {
		names = append(names, nameFn(store.DefaultTableNames))
	}
	return append(names,
		string(store.DefaultTableNames.AuditLogs()),
		string(store.DefaultTableNames.LocationGroups()),
		string(store.DefaultTableNames.Users()),
	)
}

// SQLInjectionPayloads exposes the payload corpus of the white-box injection
// property to the black-box test that runs the same payloads against
// PostgreSQL. It compiles only under `go test`.
func SQLInjectionPayloads() []string {
	return append([]string(nil), sqlInjectionPayloads...)
}

// UserTenantMoveColumns exposes how a tenant move treats each user reference
// it knows, as "table.column" keys: "owned" columns block the move, "deleted"
// and "retenanted" ones are rows the move removes or carries along. A test
// checks the lists against the live schema. It compiles only under `go test`.
func UserTenantMoveColumns() map[string]string {
	columns := make(map[string]string)
	for _, t := range userOwnedTables {
		columns[string(t.table(store.DefaultTableNames))+"."+t.userColumn] = "owned"
	}
	for _, nameFn := range userTenantMoveDeletes {
		columns[string(nameFn(store.DefaultTableNames))+".user_id"] = "deleted"
	}
	for _, nameFn := range userTenantMoveRetenants {
		columns[string(nameFn(store.DefaultTableNames))+".user_id"] = "retenanted"
	}
	return columns
}
