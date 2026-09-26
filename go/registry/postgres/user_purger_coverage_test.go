package postgres_test

import (
	"context"
	"database/sql"
	"sort"
	"testing"

	qt "github.com/frankban/quicktest"
	_ "github.com/jackc/pgx/v5/stdlib"

	"go.5x5.cz/inventario/registry/postgres"
)

// userPurgeExemptTables carry a NO ACTION foreign key on a `user_id` column and
// are deliberately absent from the purger's by-user_id list. Every entry states
// why.
var userPurgeExemptTables = map[string]string{
	// Reached through the user's jobs rather than by user_id: slots carry no
	// user_id of their own, and the pair has to be deleted child-first. See
	// purgeThumbnailChain.
	"thumbnail_generation_jobs": "purged by purgeThumbnailChain",
	"user_concurrency_slots":    "purged by purgeThumbnailChain, via the job subquery",
}

// A table with a NO ACTION foreign key to users(id) on a `user_id` column holds
// rows belonging to one user, and the final DELETE FROM users fails while any
// remain. So every such table has to be purged before a user can be deleted,
// and a table added later is exactly what gets missed — the purger's list is
// hand-written and nothing else checks it.
//
// Columns other than `user_id` are out of scope deliberately:
// `created_by_user_id` and friends mark authorship of shared content, which
// this purger must not delete. Its documented precondition is that the user no
// longer owns any.
func TestUserPurger_CoversEveryPerUserTable(t *testing.T) {
	c := qt.New(t)

	dsn := skipIfNoPostgreSQL(t)
	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	db, err := sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = db.Close() })

	rows, err := db.QueryContext(context.Background(), `
		SELECT DISTINCT c.conrelid::regclass::text
		FROM pg_constraint c
		JOIN unnest(c.conkey) k ON true
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k
		WHERE c.contype = 'f'
		  AND c.confrelid = 'users'::regclass
		  AND a.attname = 'user_id'
		  AND c.confdeltype = 'a'
		ORDER BY 1`)
	c.Assert(err, qt.IsNil)
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		c.Assert(rows.Scan(&table), qt.IsNil)
		tables = append(tables, table)
	}
	c.Assert(rows.Err(), qt.IsNil)
	// An empty list would pass every assertion below without checking anything.
	c.Assert(len(tables) > 5, qt.IsTrue,
		qt.Commentf("expected per-user tables, found %d: %v", len(tables), tables))

	purged := make(map[string]bool)
	for _, table := range postgres.UserPurgeTableNames() {
		purged[table] = true
	}

	var missing []string
	for _, table := range tables {
		if purged[table] {
			continue
		}
		if _, exempt := userPurgeExemptTables[table]; exempt {
			continue
		}
		missing = append(missing, table)
	}
	sort.Strings(missing)
	c.Check(missing, qt.HasLen, 0,
		qt.Commentf("these tables hold per-user rows that would block DELETE FROM users: %v", missing))

	// An exemption naming a table that no longer has the foreign key is a stale
	// excuse that could be hiding a real gap.
	present := make(map[string]bool, len(tables))
	for _, table := range tables {
		present[table] = true
	}
	for table, reason := range userPurgeExemptTables {
		c.Check(present[table], qt.IsTrue,
			qt.Commentf("%q is exempt (%s) but no longer has a NO ACTION user_id foreign key — drop the exemption", table, reason))
	}
}
