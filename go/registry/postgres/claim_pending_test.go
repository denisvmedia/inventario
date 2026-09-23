package postgres_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
)

// #2472: the in-process guard covers one replica. This is the half that has
// to hold across them — the condition lives in the UPDATE, so the database
// picks the winner. Worth running against a real one: an implementation that
// reads then writes passes every in-memory test and still loses here.
func TestExportRegistry_ClaimPending_Postgres(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	ctx := appctx.WithUser(context.Background(), user)

	export, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusPending,
		Description: "claim me",
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	won := raceToClaimPg(c, 32, func() (bool, error) {
		return set.ExportRegistry.ClaimPending(ctx, export.ID)
	})
	c.Assert(won, qt.Equals, int32(1), qt.Commentf("32 callers, one winner"))

	after, err := set.ExportRegistry.Get(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Check(after.Status, qt.Equals, models.ExportStatusInProgress)

	// And a second pass over the same row claims nothing: the status no
	// longer satisfies the condition.
	claimed, err := set.ExportRegistry.ClaimPending(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Check(claimed, qt.IsFalse)
}

func TestRestoreOperationRegistry_ClaimPending_Postgres(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	ctx := appctx.WithUser(context.Background(), user)

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

	won := raceToClaimPg(c, 32, func() (bool, error) {
		return set.RestoreOperationRegistry.ClaimPending(ctx, op.ID)
	})
	c.Assert(won, qt.Equals, int32(1),
		qt.Commentf("a restore replayed twice overwrites live data"))

	after, err := set.RestoreOperationRegistry.Get(ctx, op.ID)
	c.Assert(err, qt.IsNil)
	c.Check(after.Status, qt.Equals, models.RestoreStatusRunning)
}

// A row that is not pending stays where it is; a stale listing must not drag
// a finished job back into the queue.
func TestExportRegistry_ClaimPending_Postgres_OnlyFromPending(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	ctx := appctx.WithUser(context.Background(), user)

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

	claimed, err = set.ExportRegistry.ClaimPending(ctx, "00000000-0000-0000-0000-000000000000")
	c.Assert(err, qt.IsNil)
	c.Check(claimed, qt.IsFalse, qt.Commentf("a missing row is not an error, just not claimable"))
}

// raceToClaimPg fires n claims at once and returns how many reported success.
// The barrier matters: started sequentially, the first finishes before the
// second begins and the test passes whatever the implementation does.
func raceToClaimPg(c *qt.C, n int, claim func() (bool, error)) int32 {
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
