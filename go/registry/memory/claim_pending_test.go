package memory_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// #2472: the workers listed pending rows and flipped the status inside the
// goroutine they spawned, so two callers could both read `pending` and both
// proceed. The claim has to pick exactly one winner — that is the whole
// contract, and it only means anything under concurrency.
func TestExportRegistry_ClaimPending_OneWinner(t *testing.T) {
	c := qt.New(t)

	set, ctx := seedClaimRegistrySet(c)
	export, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusPending,
		Description: "claim me",
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	won := raceToClaim(c, 64, func() (bool, error) {
		return set.ExportRegistry.ClaimPending(ctx, export.ID)
	})
	c.Assert(won, qt.Equals, int32(1), qt.Commentf("64 callers, one winner"))

	after, err := set.ExportRegistry.Get(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Check(after.Status, qt.Equals, models.ExportStatusInProgress)
}

// A row that is not pending is not claimable — a completed export must not be
// dragged back into the queue by a stale listing.
func TestExportRegistry_ClaimPending_OnlyFromPending(t *testing.T) {
	c := qt.New(t)

	set, ctx := seedClaimRegistrySet(c)
	export, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "already done",
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	claimed, err := set.ExportRegistry.ClaimPending(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Check(claimed, qt.IsFalse)

	after, err := set.ExportRegistry.Get(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Check(after.Status, qt.Equals, models.ExportStatusCompleted)
}

func TestExportRegistry_ClaimPending_MissingRow(t *testing.T) {
	c := qt.New(t)

	set, ctx := seedClaimRegistrySet(c)
	claimed, err := set.ExportRegistry.ClaimPending(ctx, "no-such-export")
	c.Assert(err, qt.IsNil)
	c.Check(claimed, qt.IsFalse)
}

func TestRestoreOperationRegistry_ClaimPending_OneWinner(t *testing.T) {
	c := qt.New(t)

	set, ctx := seedClaimRegistrySet(c)
	export, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "source",
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	op, err := set.RestoreOperationRegistry.Create(ctx, models.RestoreOperation{
		ExportID:    export.ID,
		Description: "claim me",
		Status:      models.RestoreStatusPending,
		Options:     models.RestoreOptions{Strategy: "full_replace"},
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	won := raceToClaim(c, 64, func() (bool, error) {
		return set.RestoreOperationRegistry.ClaimPending(ctx, op.ID)
	})
	c.Assert(won, qt.Equals, int32(1), qt.Commentf("a restore replayed twice overwrites live data"))

	after, err := set.RestoreOperationRegistry.Get(ctx, op.ID)
	c.Assert(err, qt.IsNil)
	c.Check(after.Status, qt.Equals, models.RestoreStatusRunning)
}

// raceToClaim fires n claims at once and returns how many reported success.
// The barrier matters: without it the goroutines start far enough apart that
// the first one finishes before the second begins, and the test passes
// whatever the implementation does.
func raceToClaim(c *qt.C, n int, claim func() (bool, error)) int32 {
	c.Helper()

	var won int32
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	done.Add(n)

	for range n {
		go func() {
			defer done.Done()
			start.Wait()
			ok, err := claim()
			if err == nil && ok {
				atomic.AddInt32(&won, 1)
			}
		}()
	}
	start.Done()
	done.Wait()
	return won
}

func seedClaimRegistrySet(c *qt.C) (*registry.Set, context.Context) {
	c.Helper()

	factorySet := memory.NewFactorySet()
	tenant, err := factorySet.TenantRegistry.Create(c.Context(), models.Tenant{Name: "Claim Tenant"})
	c.Assert(err, qt.IsNil)

	user, err := factorySet.UserRegistry.Create(c.Context(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "Claim User",
		Email:               "claim@example.com",
	})
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(c.Context(), user)
	set, err := factorySet.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	return set, ctx
}
