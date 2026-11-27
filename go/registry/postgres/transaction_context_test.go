package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/registry/postgres/store"
)

func noopTx(context.Context, *sqlx.Tx) error { return nil }

// TestTransactionBegin_PoolExhausted_StopsWaitingWhenContextEnds holds the
// only connection of a one-connection pool and then opens a transaction in
// every way the registries do. Each must give up when the caller's context
// ends instead of queueing until a connection frees up (#2875).
func TestTransactionBegin_PoolExhausted_StopsWaitingWhenContextEnds(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	config, err := pgxpool.ParseConfig(dsn)
	c.Assert(err, qt.IsNil)
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(c.Context(), config)
	c.Assert(err, qt.IsNil)
	c.Cleanup(pool.Close)

	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	c.Cleanup(func() { _ = dbx.Close() })

	// A call that does not return in time is still queued when the test
	// ends. Release the held connection first so it can finish, and wait for
	// it before the pool is closed.
	var pending sync.WaitGroup
	c.Cleanup(pending.Wait)

	held, err := pool.Acquire(c.Context())
	c.Assert(err, qt.IsNil)
	c.Cleanup(held.Release)

	tenantID, userID, groupID := uuid.NewString(), uuid.NewString(), uuid.NewString()

	cases := []struct {
		name  string
		begin func(ctx context.Context) error
	}{
		{
			name: "non-RLS repository",
			begin: func(ctx context.Context) error {
				return store.NewSQLRegistry[models.User](dbx, "users").Do(ctx, noopTx)
			},
		},
		{
			name: "user-scoped repository",
			begin: func(ctx context.Context) error {
				return store.NewUserAwareSQLRegistry[models.Setting](dbx, userID, tenantID, "settings").Do(ctx, noopTx)
			},
		},
		{
			name: "group-scoped transaction",
			begin: func(ctx context.Context) error {
				return store.DoAsGroup(ctx, dbx, tenantID, groupID, noopTx)
			},
		},
		{
			name: "background-worker transaction",
			begin: func(ctx context.Context) error {
				return store.DoAsBackgroundWorker(ctx, dbx, noopTx)
			},
		},
		{
			name: "admin transaction",
			begin: func(ctx context.Context) error {
				return store.DoAsAdmin(ctx, dbx, noopTx)
			},
		},
		{
			name: "user registry create",
			begin: func(ctx context.Context) error {
				_, err := postgres.NewFactorySet(dbx).CreateServiceRegistrySet().UserRegistry.Create(ctx, models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
					Email:               "waiting@example.com",
					Name:                "Waiting",
				})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			done := make(chan error, 1)
			pending.Go(func() { done <- tc.begin(ctx) })

			select {
			case err := <-done:
				c.Assert(err, qt.ErrorIs, context.DeadlineExceeded)
			case <-time.After(10 * time.Second):
				c.Fatal("the transaction kept waiting for a connection after its context ended")
			}
		})
	}
}

// TestNonRLSRepositoryDo_ContextEndsBeforeCommit_ReportsError ends the
// context after the work has run and before the commit. database/sql rolls
// a transaction back when its context ends, so the work is gone, and the
// caller has to be told rather than handed a success.
func TestNonRLSRepositoryDo_ContextEndsBeforeCommit_ReportsError(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")

	ctx, cancel := context.WithCancel(c.Context())
	defer cancel()

	err = store.NewSQLRegistry[models.User](dbx, "users").Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, "SELECT 1"); err != nil {
			return err
		}
		cancel()
		return nil
	})
	c.Assert(err, qt.IsNotNil)
}

// TestNonRLSRepositoryDo_AbortedTransaction_ReportsError swallows a failed
// statement inside the operation. PostgreSQL turns the COMMIT of an aborted
// transaction into a ROLLBACK, so nothing was written and the caller has to
// hear about it.
func TestNonRLSRepositoryDo_AbortedTransaction_ReportsError(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")

	err = store.NewSQLRegistry[models.User](dbx, "users").Do(c.Context(), func(ctx context.Context, tx *sqlx.Tx) error {
		_, stmtErr := tx.ExecContext(ctx, "SELECT 1/0")
		c.Assert(stmtErr, qt.IsNotNil)
		return nil
	})
	c.Assert(err, qt.IsNotNil)
}
