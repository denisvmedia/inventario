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
