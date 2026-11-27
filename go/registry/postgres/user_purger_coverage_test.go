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
// Columns other than `user_id` are covered by
// TestAccountDeletion_CoversEveryUserReference.
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

// accountDeletionHandledApart are user references that account deletion
// clears with code of its own rather than from a list. Every entry says where.
var accountDeletionHandledApart = map[string]string{
	"thumbnail_generation_jobs.user_id": "removed by purgeThumbnailChain",
	"user_concurrency_slots.user_id":    "removed by purgeThumbnailChain, via the job subquery",
	"group_invites_audit.created_by":    "removed by purgeGroupInvitesAudit, in every tenant",
	"group_invites_audit.used_by":       "removed by purgeGroupInvitesAudit, in every tenant",
	"group_invites.used_by":             "removed by purgeAcceptedInvites, in every tenant",
}

// A NO ACTION foreign key to users(id) fails the final DELETE FROM users while
// any row still names the user. Account deletion therefore has to know, for
// every such column, whether a row blocks the deletion up front, is removed
// with the account, or passes to another member of its group. A column it
// does not know passes the pre-check and then fails the final delete, after
// the user's private groups, memberships and sessions are already gone. A
// table added later without a decision fails here.
func TestAccountDeletion_CoversEveryUserReference(t *testing.T) {
	c := qt.New(t)

	dsn := skipIfNoPostgreSQL(t)
	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	db, err := sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = db.Close() })

	rows, err := db.QueryContext(context.Background(), `
		SELECT c.conrelid::regclass::text || '.' || a.attname, c.confdeltype IN ('a', 'r')
		FROM pg_constraint c
		JOIN unnest(c.conkey) k ON true
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k
		WHERE c.contype = 'f' AND c.confrelid = 'users'::regclass
		ORDER BY 1`)
	c.Assert(err, qt.IsNil)
	defer rows.Close()

	present := make(map[string]bool)
	var blocking []string
	for rows.Next() {
		var ref string
		var noAction bool
		c.Assert(rows.Scan(&ref, &noAction), qt.IsNil)
		present[ref] = true
		if noAction {
			blocking = append(blocking, ref)
		}
	}
	c.Assert(rows.Err(), qt.IsNil)
	// An empty list would pass every assertion below without checking anything.
	c.Assert(len(blocking) > 20, qt.IsTrue,
		qt.Commentf("expected NO ACTION references to users, found %d: %v", len(blocking), blocking))

	known := postgres.AccountDeletionColumns()
	var missing []string
	for _, ref := range blocking {
		if _, ok := known[ref]; ok {
			continue
		}
		if _, ok := accountDeletionHandledApart[ref]; ok {
			continue
		}
		missing = append(missing, ref)
	}
	sort.Strings(missing)
	c.Check(missing, qt.HasLen, 0,
		qt.Commentf("these columns name a user but account deletion has no rule for them: %v", missing))

	for ref, rule := range known {
		c.Check(rule, qt.Not(qt.Equals), "unclassified", qt.Commentf("%s has no account deletion rule", ref))
		c.Check(present[ref], qt.IsTrue, qt.Commentf("%s is %s by account deletion but is not a foreign key to users", ref, rule))
	}
	for ref, reason := range accountDeletionHandledApart {
		c.Check(present[ref], qt.IsTrue, qt.Commentf("%s is %s but is not a foreign key to users", ref, reason))
	}
}
