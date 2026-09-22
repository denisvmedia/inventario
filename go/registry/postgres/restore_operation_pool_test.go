package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// #2469: List loaded each operation's steps from inside the cursor of the
// outer query, so every row opened a second transaction while the first was
// still walking. One call held two connections for its whole duration.
//
// A pool of one connection is the sharpest way to state that: the nested
// query waits for a connection the outer cursor will not release until the
// nested query returns, so the old code cannot finish at all. The wait is
// bounded, because a deadlock that hangs burns the test budget instead of
// reporting anything.
func TestRestoreOperationRegistry_ListDoesNotNestTransactions(t *testing.T) {
	c := qt.New(t)

	dsn := skipIfNoPostgreSQL(t)
	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	ctx := appctx.WithUser(context.Background(), user)

	// Two operations, each with two steps, so a per-row loader would issue
	// several nested queries rather than getting lucky on one.
	for i := range 2 {
		export, err := set.ExportRegistry.Create(ctx, models.Export{
			Type:        models.ExportTypeFullDatabase,
			Status:      models.ExportStatusCompleted,
			Description: "Export for pool test",
			CreatedDate: models.PNow(),
		})
		c.Assert(err, qt.IsNil)

		op, err := set.RestoreOperationRegistry.Create(ctx, models.RestoreOperation{
			ExportID:    export.ID,
			Description: "Restore for pool test",
			Status:      models.RestoreStatusCompleted,
			Options:     models.RestoreOptions{Strategy: "full_replace"},
			CreatedDate: models.PNow(),
		})
		c.Assert(err, qt.IsNil)

		for j := range 2 {
			_, err = set.RestoreStepRegistry.Create(ctx, models.RestoreStep{
				RestoreOperationID: op.ID,
				Name:               "restore-step",
				Result:             models.RestoreStepResultSuccess,
				CreatedDate:        models.PNow(),
				UpdatedDate:        models.PNow(),
			})
			c.Assert(err, qt.IsNil, qt.Commentf("operation %d step %d", i, j))
		}
	}

	singleConn := singleConnectionRegistrySet(c, dsn, user)

	// A context deadline cannot rescue this: the nested read begins its
	// transaction without one, so it waits on the pool forever. The bound has
	// to live outside the call, or the failure arrives as the package timeout
	// three minutes later instead of as a failed assertion here.
	type listResult struct {
		operations []*models.RestoreOperation
		err        error
	}
	done := make(chan listResult, 1)
	go func() {
		ops, err := singleConn.RestoreOperationRegistry.List(ctx)
		done <- listResult{ops, err}
	}()

	var got listResult
	select {
	case got = <-done:
	case <-time.After(20 * time.Second):
		c.Fatalf("List did not finish on a pool of one connection: it is holding a transaction open per row")
	}

	operations, err := got.operations, got.err
	c.Assert(err, qt.IsNil)
	c.Assert(len(operations) >= 2, qt.IsTrue,
		qt.Commentf("got %d operations", len(operations)))

	// The steps still arrive; draining the cursor first must not drop them.
	withSteps := 0
	for _, op := range operations {
		c.Check(op.Steps, qt.IsNotNil,
			qt.Commentf("Steps must be an empty slice, never nil, so a caller cannot tell none from not-loaded"))
		if len(op.Steps) > 0 {
			withSteps++
		}
	}
	c.Check(withSteps >= 2, qt.IsTrue,
		qt.Commentf("only %d operations came back with steps", withSteps))
}

// singleConnectionRegistrySet builds a registry set whose pool allows exactly
// one connection, on the same tenant and group as the shared test set.
func singleConnectionRegistrySet(c *qt.C, dsn string, user *models.User) *registry.Set {
	c.Helper()

	config, err := pgxpool.ParseConfig(dsn)
	c.Assert(err, qt.IsNil)
	config.MaxConns = 1
	config.MinConns = 1

	pool, err := pgxpool.NewWithConfig(c.Context(), config)
	c.Assert(err, qt.IsNil)
	// Closing waits for every connection to come back, and the whole point of
	// this pool is that a regression leaves one out. Let the process reclaim it.
	c.Cleanup(func() { go pool.Close() })

	sqlxDB := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")

	// The group the shared set seeded; read through it rather than assuming an id.
	groups, err := postgres.NewRegistrySetWithUserAndGroupID(sqlxDB, user.ID, user.TenantID, "").
		LocationGroupRegistry.List(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.Not(qt.HasLen), 0)

	return postgres.NewRegistrySetWithUserAndGroupID(sqlxDB, user.ID, user.TenantID, groups[0].ID)
}
