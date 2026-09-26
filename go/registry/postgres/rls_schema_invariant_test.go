package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A tenant-scoped table without RLS, or with RLS and no policy, is a table
// every tenant can read — and neither shows up as a failing query, the rows
// just come back. The table list comes from the live schema so a new table
// cannot miss this by being absent from a list.

// tenantPredicate is what a policy has to consult to be tenant-scoped at all.
// Matching the call rather than the whole expression keeps this from becoming a
// second copy of the annotations: the shape differs per table (some add a group
// or a user), the tenant leg does not.
const tenantPredicate = "get_current_tenant_id()"

// rlsExemptTables are the tenant_id-carrying tables that run without RLS on
// purpose. Every entry carries its reason.
var rlsExemptTables = map[string]string{
	// The tenant is what the token resolves to, so the lookup that
	// establishes it cannot carry a tenant predicate.
	"email_verifications": "token lookup precedes tenant context",
	"magic_link_tokens":   "token lookup precedes tenant context",
	"password_resets":     "token lookup precedes tenant context",
}

// tenantScopedTables returns the public tables carrying a tenant_id column,
// which is what makes a table tenant-scoped here.
func tenantScopedTables(c *qt.C, pool *pgxpool.Pool) []string {
	c.Helper()

	rows, err := pool.Query(context.Background(), `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN information_schema.columns col
		  ON col.table_schema = n.nspname AND col.table_name = c.relname
		WHERE n.nspname = 'public'
		  AND c.relkind = 'r'
		  AND col.column_name = 'tenant_id'
		ORDER BY c.relname`)
	c.Assert(err, qt.IsNil)
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		c.Assert(rows.Scan(&name), qt.IsNil)
		tables = append(tables, name)
	}
	c.Assert(rows.Err(), qt.IsNil)
	return tables
}

func TestSchema_EveryTenantScopedTableIsProtectedByRLS(t *testing.T) {
	c := qt.New(t)

	dsn := skipIfNoPostgreSQL(t)
	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	pool, err := pgxpool.New(t.Context(), dsn)
	c.Assert(err, qt.IsNil)
	defer pool.Close()

	tables := tenantScopedTables(c, pool)
	// An empty list would pass every assertion below without running one.
	c.Assert(len(tables) > 10, qt.IsTrue,
		qt.Commentf("expected tenant-scoped tables, found %d: %v", len(tables), tables))

	present := make(map[string]bool, len(tables))
	for _, table := range tables {
		present[table] = true
	}
	for table := range rlsExemptTables {
		c.Check(present[table], qt.IsTrue,
			qt.Commentf("%q is exempt from RLS but no longer carries tenant_id — drop the exemption", table))
	}

	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			c := qt.New(t)

			if reason, exempt := rlsExemptTables[table]; exempt {
				t.Skipf("exempt: %s", reason)
			}

			var enabled bool
			err := pool.QueryRow(t.Context(), `
				SELECT c.relrowsecurity
				FROM pg_class c
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE n.nspname = 'public' AND c.relname = $1`, table).Scan(&enabled)
			c.Assert(err, qt.IsNil)
			c.Check(enabled, qt.IsTrue,
				qt.Commentf("row-level security is off, so every tenant reads this table"))

			// FORCE ROW LEVEL SECURITY is not asserted: no table carries it,
			// and bootstrap gives the tables to the migration role, so the
			// owner-bypass never reaches the application. #2633 covers whether
			// a single-role install is supported.

			// Counting policies cannot see the regression this test is for.
			// Every table here also carries a background-worker policy with
			// USING (true), so a count stays positive when the application
			// policy is dropped or weakened. Permissive policies are OR-ed,
			// so one wide-open policy beside a scoped one is the whole table.
			rows, err := pool.Query(t.Context(), `
				SELECT policyname, COALESCE(qual, ''), COALESCE(with_check, '')
				FROM pg_policies
				WHERE schemaname = 'public'
				  AND tablename = $1
				  AND permissive = 'PERMISSIVE'
				  AND roles && ARRAY['inventario_app', 'public']::name[]`, table)
			c.Assert(err, qt.IsNil)
			defer rows.Close()

			appPolicies := 0
			for rows.Next() {
				var name, qual, withCheck string
				c.Assert(rows.Scan(&name, &qual, &withCheck), qt.IsNil)
				appPolicies++

				c.Check(qual, qt.Contains, tenantPredicate,
					qt.Commentf("policy %q returns rows with no tenant predicate: USING (%s)", name, qual))
				// NULL for a policy that governs reads only.
				if withCheck != "" {
					c.Check(withCheck, qt.Contains, tenantPredicate,
						qt.Commentf("policy %q accepts writes with no tenant predicate: WITH CHECK (%s)", name, withCheck))
				}
			}
			c.Assert(rows.Err(), qt.IsNil)
			c.Check(appPolicies > 0, qt.IsTrue,
				qt.Commentf("RLS is on with no policy for the application role"))
		})
	}
}
