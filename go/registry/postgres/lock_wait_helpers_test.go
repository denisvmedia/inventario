package postgres_test

import (
	"context"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jmoiron/sqlx"
)

// holdRowForUpdate locks one row of table in a transaction of its own and
// returns the function that commits it. The tests use it to line two writers
// up on the same row before either of them runs on.
func holdRowForUpdate(c *qt.C, dbx *sqlx.DB, table, id string) (release func()) {
	c.Helper()
	tx, err := dbx.BeginTxx(context.Background(), nil)
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(context.Background(), `SELECT 1 FROM `+table+` WHERE id = $1 FOR UPDATE`, id)
	c.Assert(err, qt.IsNil)
	return func() { c.Assert(tx.Commit(), qt.IsNil) }
}

// waitForLockWaiters returns once n sessions of the test database wait on a
// lock. It fails the test when one of done delivers first: that call ran to
// the end without waiting.
func waitForLockWaiters(c *qt.C, dbx *sqlx.DB, n int, done ...<-chan error) {
	c.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		var waiting int
		c.Assert(dbx.GetContext(context.Background(), &waiting,
			`SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`), qt.IsNil)
		if waiting >= n {
			return
		}
		for _, ch := range done {
			select {
			case err := <-ch:
				c.Fatalf("the call returned before it waited on a lock: %v", err)
			default:
			}
		}
		if time.Now().After(deadline) {
			c.Fatalf("%d sessions never waited on a lock at once", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
