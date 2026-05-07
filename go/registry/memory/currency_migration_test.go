package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// setupCurrencyMigrationRegistry seeds a tenant + group + user and
// returns the user-mode CurrencyMigrationRegistry plus the seeded group
// id (callers usually want the group id for InFlightForGroup-style
// queries).
func setupCurrencyMigrationRegistry(c *qt.C) (userReg, serviceReg registry.CurrencyMigrationRegistry, groupID string) {
	c.Helper()
	factorySet := memory.NewFactorySet()

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
		Email: "u@example.com",
		Name:  "User",
	}
	u, err := factorySet.CreateServiceRegistrySet().UserRegistry.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	group := &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: u.TenantID,
		},
		Slug:          "default-group-default-slug",
		Name:          "Default",
		GroupCurrency: "USD",
	}
	ctx := appctx.WithUser(context.Background(), u)
	ctx = appctx.WithGroup(ctx, group)

	regSet := must.Must(factorySet.CreateUserRegistrySet(ctx))
	serviceSet := factorySet.CreateServiceRegistrySet()
	return regSet.CurrencyMigrationRegistry, serviceSet.CurrencyMigrationRegistry, group.ID
}

func TestCurrencyMigrationRegistry_Create_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	userReg, _, groupID := setupCurrencyMigrationRegistry(c)

	op, err := userReg.Create(ctx, models.CurrencyMigration{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			GroupID: groupID,
		},
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.92),
	})

	c.Assert(err, qt.IsNil)
	c.Assert(op.Status, qt.Equals, models.CurrencyMigrationStatusPending)
	c.Assert(op.GroupID, qt.Equals, groupID)
	c.Assert(op.ID, qt.Not(qt.Equals), "")
	c.Assert(op.UUID, qt.Not(qt.Equals), "")
}

func TestCurrencyMigrationRegistry_Create_ParallelInFlightRejected(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	userReg, _, groupID := setupCurrencyMigrationRegistry(c)

	first, err := userReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.92),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(first, qt.IsNotNil)

	// Second pending row for the same group is rejected.
	_, err = userReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "GBP",
		ExchangeRate: decimal.NewFromFloat(0.79),
	})
	c.Assert(err, qt.ErrorIs, registry.ErrMigrationInFlight)
	_ = groupID
}

func TestCurrencyMigrationRegistry_Validate_RejectsSameCurrency(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	userReg, _, _ := setupCurrencyMigrationRegistry(c)

	_, err := userReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "USD",
		ExchangeRate: decimal.NewFromInt(1),
	})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "from and to currencies must differ")
}

func TestCurrencyMigrationRegistry_Validate_RejectsBadRate(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	userReg, _, _ := setupCurrencyMigrationRegistry(c)

	for _, rate := range []decimal.Decimal{decimal.Zero, decimal.NewFromInt(-1)} {
		_, err := userReg.Create(ctx, models.CurrencyMigration{
			FromCurrency: "USD",
			ToCurrency:   "EUR",
			ExchangeRate: rate,
		})
		c.Assert(err, qt.IsNotNil, qt.Commentf("rate=%s", rate.String()))
	}
}

func TestCurrencyMigrationRegistry_InFlightAndDailyCap(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	userReg, serviceReg, groupID := setupCurrencyMigrationRegistry(c)

	// Initially nothing is in flight.
	inFlight, err := userReg.InFlightForGroup(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(inFlight, qt.IsNil)

	// Insert pending → InFlightForGroup returns it.
	op, err := userReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.9),
	})
	c.Assert(err, qt.IsNil)

	inFlight, err = userReg.InFlightForGroup(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(inFlight, qt.IsNotNil)
	c.Assert(inFlight.ID, qt.Equals, op.ID)

	// Worker runs to completion: pending → running → completed.
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	c.Assert(serviceReg.UpdateStatus(ctx, op.ID, registry.CurrencyMigrationStatusPatch{
		Status:    models.CurrencyMigrationStatusRunning,
		StartedAt: new(now),
	}), qt.IsNil)
	c.Assert(serviceReg.UpdateStatus(ctx, op.ID, registry.CurrencyMigrationStatusPatch{
		Status:      models.CurrencyMigrationStatusCompleted,
		CompletedAt: new(now.Add(time.Second)),
	}), qt.IsNil)

	// Once completed, no longer in-flight.
	inFlight, err = userReg.InFlightForGroup(ctx, groupID)
	c.Assert(err, qt.IsNil)
	c.Assert(inFlight, qt.IsNil)

	// Daily-cap query reflects the freshly-completed row.
	cnt, err := userReg.CompletedTodayForGroup(ctx, groupID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, 1)

	// New day → cap reset.
	tomorrow := now.Add(24 * time.Hour)
	cnt, err = userReg.CompletedTodayForGroup(ctx, groupID, tomorrow)
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, 0)
}

func TestCurrencyMigrationRegistry_ClaimNextPending_ServiceOnly(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	userReg, serviceReg, _ := setupCurrencyMigrationRegistry(c)

	created, err := userReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromInt(1),
	})
	c.Assert(err, qt.IsNil)

	// User mode cannot claim — guard rail for the worker.
	_, err = userReg.ClaimNextPending(ctx)
	c.Assert(err, qt.IsNotNil)

	// Service registry claims the row and flips it to running.
	picked, err := serviceReg.ClaimNextPending(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(picked.ID, qt.Equals, created.ID)
	c.Assert(picked.Status, qt.Equals, models.CurrencyMigrationStatusRunning)
	c.Assert(picked.StartedAt, qt.IsNotNil)

	// Second claim returns ErrNotFound — nothing left in pending.
	_, err = serviceReg.ClaimNextPending(ctx)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestCurrencyMigrationRegistry_SweepStuckRunning(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	userReg, serviceReg, _ := setupCurrencyMigrationRegistry(c)

	created, err := userReg.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromInt(1),
	})
	c.Assert(err, qt.IsNil)
	long := time.Date(2026, 5, 7, 8, 0, 0, 0, time.UTC)
	c.Assert(serviceReg.UpdateStatus(ctx, created.ID, registry.CurrencyMigrationStatusPatch{
		Status:    models.CurrencyMigrationStatusRunning,
		StartedAt: new(long),
	}), qt.IsNil)

	// Sweep runs at long+30m with threshold=10m → row is stuck → fails.
	now := long.Add(30 * time.Minute)
	swept, err := serviceReg.SweepStuckRunning(ctx, now, 10*time.Minute)
	c.Assert(err, qt.IsNil)
	c.Assert(swept, qt.HasLen, 1)
	c.Assert(swept[0].Status, qt.Equals, models.CurrencyMigrationStatusFailed)
	c.Assert(swept[0].CompletedAt, qt.IsNotNil)
	c.Assert(swept[0].ErrorMessage, qt.Contains, "worker crashed")

	// Failed runs do NOT count toward the daily cap.
	cnt, err := userReg.CompletedTodayForGroup(ctx, swept[0].GroupID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, 0)
}

func TestCurrencyMigrationRegistry_PreviewToken_RoundTrip(t *testing.T) {
	c := qt.New(t)
	userReg, _, groupID := setupCurrencyMigrationRegistry(c)

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	inputs := registry.PreviewTokenInputs{
		GroupID:      groupID,
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		Rate:         "0.92",
		StateHash:    "deadbeef",
		ExpiresAt:    now.Add(10 * time.Minute),
	}
	token, err := userReg.IssuePreviewToken(inputs)
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	// Verify within TTL → success.
	out, err := userReg.VerifyPreviewToken(token, now.Add(5*time.Minute))
	c.Assert(err, qt.IsNil)
	c.Assert(out.GroupID, qt.Equals, inputs.GroupID)
	c.Assert(out.Rate, qt.Equals, "0.92")
	c.Assert(out.StateHash, qt.Equals, "deadbeef")

	// Verify after TTL → ErrPreviewTokenExpired.
	_, err = userReg.VerifyPreviewToken(token, now.Add(11*time.Minute))
	c.Assert(err, qt.ErrorIs, registry.ErrPreviewTokenExpired)

	// Tampered token → ErrPreviewTokenInvalid.
	_, err = userReg.VerifyPreviewToken(token+"x", now.Add(time.Minute))
	c.Assert(err, qt.ErrorIs, registry.ErrPreviewTokenInvalid)
	_, err = userReg.VerifyPreviewToken("garbage.notaken", now.Add(time.Minute))
	c.Assert(err, qt.ErrorIs, registry.ErrPreviewTokenInvalid)
}

func TestCurrencyMigrationRegistry_PreviewToken_DifferentKeyRejects(t *testing.T) {
	c := qt.New(t)

	a := memory.NewCurrencyMigrationRegistryFactoryWithKey([]byte("key-a-key-a-key-a-key-a-key-a-32"))
	b := memory.NewCurrencyMigrationRegistryFactoryWithKey([]byte("key-b-key-b-key-b-key-b-key-b-32"))

	regA := a.CreateServiceRegistry()
	regB := b.CreateServiceRegistry()

	now := time.Now()
	token, err := regA.IssuePreviewToken(registry.PreviewTokenInputs{
		GroupID:   "g",
		Rate:      "1.0",
		ExpiresAt: now.Add(time.Minute),
	})
	c.Assert(err, qt.IsNil)

	_, err = regB.VerifyPreviewToken(token, now)
	c.Assert(err, qt.ErrorIs, registry.ErrPreviewTokenInvalid)
}
