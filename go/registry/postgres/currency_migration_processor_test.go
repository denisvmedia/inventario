package postgres_test

import (
	"context"
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

// TestCurrencyMigrationProcessor_HappyPath_SmallGroup walks the worker
// TX2 lifecycle end-to-end against postgres: seed two commodities,
// insert+claim a pending migration, run ProcessRunningMigration, and
// assert: commodity rows updated; migration row marked completed with
// totals; group's GroupCurrency flipped + currency_migration_id
// cleared; one audit row per commodity; one audit_logs row keyed to
// the migration; one price_changed CommodityEvent per row.
func TestCurrencyMigrationProcessor_HappyPath_SmallGroup(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	user := getTestUser(c, registrySet)
	groupID := getTestGroupID(c, registrySet, user)

	// Seed: one location, one area, two commodities (Case A both:
	// OriginalPriceCurrency=USD = group's current GroupCurrency).
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	com1 := createTestCommodity(c, registrySet, area.ID)

	// Seed a second commodity with a known different price so the
	// totals math is non-trivial.
	com2 := models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        user.TenantID,
			CreatedByUserID: user.ID,
		},
		Name:                   "Second Commodity",
		ShortName:              "SC",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(50.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.NewFromFloat(60.00),
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2023-01-01"),
		RegisteredDate:         models.ToPDate("2023-01-02"),
		LastModifiedDate:       models.ToPDate("2023-01-03"),
	}
	createdCom2, err := registrySet.CommodityRegistry.Create(ctx, com2)
	c.Assert(err, qt.IsNil)

	// Build a separate processor + service registry so the worker
	// surface is exercised exactly the way bootstrap wires it.
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	pgFactory := postgres.NewCurrencyMigrationRegistry(dbx)
	processor := pgFactory.NewProcessor()
	serviceReg := pgFactory.CreateServiceRegistry()

	// Pending row: USD → EUR at 0.9.
	created, err := registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.9),
	})
	c.Assert(err, qt.IsNil)

	// TX1 — service registry flips it to running.
	op, err := serviceReg.ClaimNextPending(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(op.ID, qt.Equals, created.ID)
	c.Assert(op.Status, qt.Equals, models.CurrencyMigrationStatusRunning)

	// TX2 — the processor.
	summary, err := processor.ProcessRunningMigration(ctx, op)
	c.Assert(err, qt.IsNil)
	c.Assert(summary.CommodityCount, qt.Equals, 2)
	c.Assert(summary.TotalBefore.String(), qt.Equals, "150") // 90 + 60
	c.Assert(summary.TotalAfter.String(), qt.Equals, "135")  // 81 + 54
	c.Assert(summary.AcquisitionFillsCount, qt.Equals, 2)    // both Case A, both NULL pre-migration
	c.Assert(summary.Duration > 0, qt.IsTrue)

	// The migration row is now completed.
	round, err := registrySet.CurrencyMigrationRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(round.Status, qt.Equals, models.CurrencyMigrationStatusCompleted)
	c.Assert(round.CommodityCount, qt.Equals, 2)
	c.Assert(round.CompletedAt, qt.IsNotNil)
	c.Assert(round.TotalBefore, qt.IsNotNil)
	c.Assert(round.TotalBefore.Equal(decimal.NewFromInt(150)), qt.IsTrue)
	c.Assert(round.TotalAfter, qt.IsNotNil)
	c.Assert(round.TotalAfter.Equal(decimal.NewFromInt(135)), qt.IsTrue)

	// Group currency flipped + lock cleared.
	updatedGroup, err := registrySet.LocationGroupRegistry.Get(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(string(updatedGroup.GroupCurrency), qt.Equals, "EUR")
	c.Assert(updatedGroup.CurrencyMigrationID, qt.IsNil)

	// Commodities updated: original_price * 0.9, current_price * 0.9,
	// converted_original_price collapsed to 0.
	roundCom1, err := registrySet.CommodityRegistry.Get(ctx, com1.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(roundCom1.OriginalPrice.String(), qt.Equals, "90")
	c.Assert(string(roundCom1.OriginalPriceCurrency), qt.Equals, "EUR")
	c.Assert(roundCom1.CurrentPrice.String(), qt.Equals, "81")
	c.Assert(roundCom1.AcquisitionPrice, qt.IsNotNil)
	c.Assert(roundCom1.AcquisitionPrice.String(), qt.Equals, "100")
	c.Assert(roundCom1.AcquisitionCurrency, qt.IsNotNil)
	c.Assert(string(*roundCom1.AcquisitionCurrency), qt.Equals, "USD")

	roundCom2, err := registrySet.CommodityRegistry.Get(ctx, createdCom2.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(roundCom2.OriginalPrice.String(), qt.Equals, "45")
	c.Assert(roundCom2.CurrentPrice.String(), qt.Equals, "54")
	c.Assert(roundCom2.AcquisitionPrice, qt.IsNotNil)
	c.Assert(roundCom2.AcquisitionPrice.String(), qt.Equals, "50")

	// Audit rows — one per commodity.
	auditRows, err := registrySet.CurrencyMigrationRegistry.ListAuditRows(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(auditRows, qt.HasLen, 2)
	for _, row := range auditRows {
		c.Assert(row.AcquisitionFilledInThisRun, qt.IsTrue)
	}

	// Commodity events — one price_changed per commodity. The
	// registry's listing is paginated; 50 is plenty for a two-row test.
	events, _, err := registrySet.CommodityEventRegistry.ListByCommodity(ctx, com1.ID, 0, 50, registry.CommodityEventListOptions{})
	c.Assert(err, qt.IsNil)
	priceChanged := 0
	for _, e := range events {
		if e.Kind == models.CommodityEventKindPriceChanged {
			priceChanged++
		}
	}
	c.Assert(priceChanged, qt.Equals, 1)

	// audit_logs has one currency_migration.complete row keyed to the
	// migration. We list newest-first and find the matching action.
	logs, err := registrySet.AuditLogRegistry.ListByAction(ctx, postgres.AuditActionCurrencyMigrationComplete)
	c.Assert(err, qt.IsNil)
	matched := 0
	for _, log := range logs {
		if log.EntityID != nil && *log.EntityID == created.ID {
			matched++
			c.Assert(log.Success, qt.IsTrue)
		}
	}
	c.Assert(matched, qt.Equals, 1)
}

// TestCurrencyMigrationProcessor_ForwardThenInverse round-trips the
// conversion: G_old → G_new at r, then G_new → G_old at 1/r. The final
// CurrentPrice must equal the starting CurrentPrice (within rounding)
// and the acquisition columns must remain at the values fixed on the
// first migration — write-once invariant from #1550.
func TestCurrencyMigrationProcessor_ForwardThenInverse(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	user := getTestUser(c, registrySet)
	groupID := getTestGroupID(c, registrySet, user)

	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	com := createTestCommodity(c, registrySet, area.ID)
	startingOriginalPrice := com.OriginalPrice
	startingCurrentPrice := com.CurrentPrice
	startingCurrency := com.OriginalPriceCurrency

	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	pgFactory := postgres.NewCurrencyMigrationRegistry(dbx)
	processor := pgFactory.NewProcessor()
	serviceReg := pgFactory.CreateServiceRegistry()

	// Forward: USD → EUR @ 0.5.
	forwardOp, err := registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.5),
	})
	c.Assert(err, qt.IsNil)
	claimed, err := serviceReg.ClaimNextPending(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed.ID, qt.Equals, forwardOp.ID)
	_, err = processor.ProcessRunningMigration(ctx, claimed)
	c.Assert(err, qt.IsNil)

	mid, err := registrySet.CommodityRegistry.Get(ctx, com.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(string(mid.OriginalPriceCurrency), qt.Equals, "EUR")
	c.Assert(mid.AcquisitionPrice, qt.IsNotNil)
	c.Assert(mid.AcquisitionPrice.Equal(startingOriginalPrice), qt.IsTrue)
	c.Assert(string(*mid.AcquisitionCurrency), qt.Equals, string(startingCurrency))

	// Inverse: EUR → USD @ 2.0 (1/0.5).
	inverseOp, err := registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		FromCurrency: "EUR",
		ToCurrency:   "USD",
		ExchangeRate: decimal.NewFromFloat(2.0),
	})
	c.Assert(err, qt.IsNil)
	claimed2, err := serviceReg.ClaimNextPending(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed2.ID, qt.Equals, inverseOp.ID)
	_, err = processor.ProcessRunningMigration(ctx, claimed2)
	c.Assert(err, qt.IsNil)

	// Final state matches starting state for the price columns.
	final, err := registrySet.CommodityRegistry.Get(ctx, com.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(final.OriginalPrice.String(), qt.Equals, startingOriginalPrice.String())
	c.Assert(string(final.OriginalPriceCurrency), qt.Equals, string(startingCurrency))
	c.Assert(final.CurrentPrice.String(), qt.Equals, startingCurrentPrice.String())

	// Acquisition columns are still the originals — the second
	// migration must NOT have overwritten them (write-once).
	c.Assert(final.AcquisitionPrice, qt.IsNotNil)
	c.Assert(final.AcquisitionPrice.Equal(startingOriginalPrice), qt.IsTrue)
	c.Assert(string(*final.AcquisitionCurrency), qt.Equals, string(startingCurrency))

	// Group currency flipped back to USD.
	finalGroup, err := registrySet.LocationGroupRegistry.Get(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(string(finalGroup.GroupCurrency), qt.Equals, "USD")
}

// TestCurrencyMigrationProcessor_RecoverySweepWithAuditLog asserts the
// post-sweep failure path: a stuck running row passes the threshold,
// the registry's SweepStuckRunning returns the swept row, and the
// processor's WriteSweepFailureAuditLog inserts a matching audit log
// keyed to the migration.
func TestCurrencyMigrationProcessor_RecoverySweepWithAuditLog(t *testing.T) {
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	user := getTestUser(c, registrySet)
	_ = user

	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	pgFactory := postgres.NewCurrencyMigrationRegistry(dbx)
	processor := pgFactory.NewProcessor()
	serviceReg := pgFactory.CreateServiceRegistry()

	created, err := registrySet.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.9),
	})
	c.Assert(err, qt.IsNil)

	// Manually flip to running with an old started_at so the sweep's
	// "started_at < cutoff" predicate matches. Cutoff = now - 10m.
	pastStart := time.Now().UTC().Add(-1 * time.Hour)
	c.Assert(serviceReg.UpdateStatus(ctx, created.ID, registry.CurrencyMigrationStatusPatch{
		Status:    models.CurrencyMigrationStatusRunning,
		StartedAt: &pastStart,
	}), qt.IsNil)

	// Sweep with the production threshold.
	swept, err := serviceReg.SweepStuckRunning(ctx, time.Now().UTC(), postgres.CurrencyMigrationStuckThreshold)
	c.Assert(err, qt.IsNil)
	c.Assert(swept, qt.HasLen, 1)
	c.Assert(swept[0].ID, qt.Equals, created.ID)
	c.Assert(swept[0].Status, qt.Equals, models.CurrencyMigrationStatusFailed)

	// Write the recovery audit log via the processor.
	c.Assert(processor.WriteSweepFailureAuditLog(ctx, swept[0]), qt.IsNil)

	// The migration row is failed with the worker-stalled message.
	round, err := registrySet.CurrencyMigrationRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(round.Status, qt.Equals, models.CurrencyMigrationStatusFailed)
	c.Assert(round.ErrorMessage, qt.Equals, "worker crashed or stalled")

	// audit_logs has the corresponding fail entry.
	logs, err := registrySet.AuditLogRegistry.ListByAction(ctx, postgres.AuditActionCurrencyMigrationFail)
	c.Assert(err, qt.IsNil)
	matched := 0
	for _, log := range logs {
		if log.EntityID != nil && *log.EntityID == created.ID {
			matched++
			c.Assert(log.Success, qt.IsFalse)
		}
	}
	c.Assert(matched, qt.Equals, 1)
}
