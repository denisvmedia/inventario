package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// TestCurrencyMigrationRegistry_Postgres_HappyPath exercises the
// pending → running → completed transitions and verifies that the
// daily-cap query, in-flight query, and audit-row write all return
// the expected results against a real Postgres backend.
func TestCurrencyMigrationRegistry_Postgres_HappyPath(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	user := getTestUser(c, registrySet)
	groupID := getTestGroupID(c, registrySet, user)

	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")

	// Build a separate factory so we can grab a service registry that
	// the worker uses (TX1/TX2 simulation).
	factory := postgres.NewCurrencyMigrationRegistry(dbx)
	serviceReg := factory.CreateServiceRegistry()

	created, err := registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.92),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.Status, qt.Equals, models.CurrencyMigrationStatusPending)

	// In-flight query sees the pending row.
	inFlight, err := registrySet.CurrencyMigrationRegistry.InFlightForGroup(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(inFlight, qt.IsNotNil)
	c.Assert(inFlight.ID, qt.Equals, created.ID)

	// Worker flips to running, then to completed.
	now := time.Now().UTC()
	c.Assert(serviceReg.UpdateStatus(ctx, created.ID, registry.CurrencyMigrationStatusPatch{
		Status:    models.CurrencyMigrationStatusRunning,
		StartedAt: &now,
	}), qt.IsNil)
	cnt := 1
	completed := now.Add(time.Second)
	c.Assert(serviceReg.UpdateStatus(ctx, created.ID, registry.CurrencyMigrationStatusPatch{
		Status:         models.CurrencyMigrationStatusCompleted,
		CompletedAt:    &completed,
		CommodityCount: &cnt,
	}), qt.IsNil)

	// Daily cap registers the new completed row.
	dailyCnt, err := registrySet.CurrencyMigrationRegistry.CompletedTodayForGroup(ctx, groupID, completed)
	c.Assert(err, qt.IsNil)
	c.Assert(dailyCnt, qt.Equals, 1)

	// Re-fetch and verify status fields.
	round, err := registrySet.CurrencyMigrationRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(round.Status, qt.Equals, models.CurrencyMigrationStatusCompleted)
	c.Assert(round.CompletedAt, qt.IsNotNil)
	c.Assert(round.CommodityCount, qt.Equals, 1)
}

// TestCurrencyMigrationRegistry_Postgres_ParallelInFlightUniqueViolation
// is the integration test mandated by issue #1550 §4: two parallel raw
// inserts with the same group_id and status='pending' must result in
// exactly one success and one ErrMigrationInFlight (mapped from
// Postgres SQLState 23505 on the partial unique index).
func TestCurrencyMigrationRegistry_Postgres_ParallelInFlightUniqueViolation(t *testing.T) {
	skipIfNoPostgreSQL(t)
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()

	// Fire both Create calls concurrently. We don't strictly need
	// goroutines (the partial unique index serialises at INSERT time
	// anyway), but doing it concurrently catches accidental
	// application-level check-then-act regressions: a check-then-act
	// could pass both check phases and then both INSERT phases would
	// try to write a pending row.
	type result struct {
		op  *models.CurrencyMigration
		err error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			op, err := registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
				FromCurrency: "USD",
				ToCurrency:   "EUR",
				ExchangeRate: decimal.NewFromFloat(0.92),
			})
			results <- result{op: op, err: err}
		}()
	}
	wg.Wait()
	close(results)

	var succeeded int
	var conflictErrors int
	for r := range results {
		switch {
		case r.err == nil:
			c.Assert(r.op, qt.IsNotNil)
			c.Assert(r.op.Status, qt.Equals, models.CurrencyMigrationStatusPending)
			succeeded++
		case errors.Is(r.err, registry.ErrMigrationInFlight):
			conflictErrors++
		default:
			c.Fatalf("unexpected error: %v", r.err)
		}
	}
	c.Assert(succeeded, qt.Equals, 1, qt.Commentf("exactly one INSERT must succeed"))
	c.Assert(conflictErrors, qt.Equals, 1, qt.Commentf("the loser must surface ErrMigrationInFlight"))
}

// TestCurrencyMigrationRegistry_Postgres_FailedRunsDontCount verifies
// that a failed run does not consume daily-cap quota — only completed
// rows count.
func TestCurrencyMigrationRegistry_Postgres_FailedRunsDontCount(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	user := getTestUser(c, registrySet)
	groupID := getTestGroupID(c, registrySet, user)

	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	serviceReg := postgres.NewCurrencyMigrationRegistry(dbx).CreateServiceRegistry()

	// Migration #1 fails.
	created, err := registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.92),
	})
	c.Assert(err, qt.IsNil)
	now := time.Now().UTC()
	c.Assert(serviceReg.UpdateStatus(ctx, created.ID, registry.CurrencyMigrationStatusPatch{
		Status:    models.CurrencyMigrationStatusRunning,
		StartedAt: &now,
	}), qt.IsNil)
	em := "boom"
	completedFail := now.Add(time.Second)
	c.Assert(serviceReg.UpdateStatus(ctx, created.ID, registry.CurrencyMigrationStatusPatch{
		Status:       models.CurrencyMigrationStatusFailed,
		CompletedAt:  &completedFail,
		ErrorMessage: &em,
	}), qt.IsNil)

	// Daily cap remains 0 — failed runs do not count.
	dailyCnt, err := registrySet.CurrencyMigrationRegistry.CompletedTodayForGroup(ctx, groupID, completedFail)
	c.Assert(err, qt.IsNil)
	c.Assert(dailyCnt, qt.Equals, 0)
}

// getTestGroupID returns the seeded test group ID for the test user.
func getTestGroupID(c *qt.C, set *registry.Set, user *models.User) string {
	c.Helper()
	groups, err := set.LocationGroupRegistry.ListByTenant(context.Background(), user.TenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.Not(qt.HasLen), 0)
	return groups[0].ID
}
