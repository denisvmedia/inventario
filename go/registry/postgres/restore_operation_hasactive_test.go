package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
)

// #1314: HasActive answers with one query that stops at the first match,
// rather than reading every operation and its steps to learn one bit.
func TestRestoreOperationRegistry_HasActive(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)
	ctx := appctx.WithUser(context.Background(), user)

	// Nothing seeded yet for this fresh schema.
	active, err := set.RestoreOperationRegistry.HasActive(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(active, qt.IsFalse)

	export, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "Export for HasActive",
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	newOp := func(status models.RestoreStatus) *models.RestoreOperation {
		op, err := set.RestoreOperationRegistry.Create(ctx, models.RestoreOperation{
			ExportID:    export.ID,
			Description: "restore " + string(status),
			Status:      status,
			Options:     models.RestoreOptions{Strategy: "full_replace"},
			CreatedDate: models.PNow(),
		})
		c.Assert(err, qt.IsNil)
		return op
	}

	// A finished operation is not active.
	done := newOp(models.RestoreStatusCompleted)
	active, err = set.RestoreOperationRegistry.HasActive(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(active, qt.IsFalse, qt.Commentf("a completed operation is not active"))

	pending := newOp(models.RestoreStatusPending)
	active, err = set.RestoreOperationRegistry.HasActive(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(active, qt.IsTrue, qt.Commentf("pending counts as active"))

	// And running does too, with the pending one gone.
	c.Assert(set.RestoreOperationRegistry.Delete(ctx, pending.ID), qt.IsNil)
	running := newOp(models.RestoreStatusRunning)
	active, err = set.RestoreOperationRegistry.HasActive(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(active, qt.IsTrue, qt.Commentf("running counts as active"))

	c.Assert(set.RestoreOperationRegistry.Delete(ctx, running.ID), qt.IsNil)
	c.Assert(set.RestoreOperationRegistry.Delete(ctx, done.ID), qt.IsNil)
	active, err = set.RestoreOperationRegistry.HasActive(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(active, qt.IsFalse)
}
