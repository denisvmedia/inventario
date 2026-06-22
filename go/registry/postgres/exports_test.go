package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// TestExportRegistry_Postgres_DeleteClearsRestorePipeline is the #2118
// regression: restore_operations.export_id is a NOT NULL NO ACTION FK to
// exports(id), and restore_steps.restore_operation_id is a NO ACTION FK to
// restore_operations(id). Before the fix, ExportRegistry.Delete issued a
// bare DELETE FROM exports, which tripped the FK and 500'd for any user who
// had run a restore from that export. The fix clears the referencing restore
// pipeline (steps, then operations) inside the same transaction before the
// export row is removed. This test seeds an export with a restore_operation
// and a restore_step, then asserts Delete succeeds (no FK error) and all
// three rows are gone.
func TestExportRegistry_Postgres_DeleteClearsRestorePipeline(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)

	ctx := appctx.WithUser(context.Background(), user)

	// Seed an export.
	export, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "Export with restore pipeline",
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	// Seed a restore_operation referencing the export (the NO ACTION FK that
	// blocks the bare DELETE FROM exports).
	restoreOp, err := set.RestoreOperationRegistry.Create(ctx, models.RestoreOperation{
		ExportID:    export.ID,
		Description: "Restore from export",
		Status:      models.RestoreStatusCompleted,
		Options: models.RestoreOptions{
			Strategy: "full_replace",
		},
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	// Seed a restore_step referencing the operation (the deepest child, whose
	// own NO ACTION FK means it must be cleared first).
	restoreStep, err := set.RestoreStepRegistry.Create(ctx, models.RestoreStep{
		RestoreOperationID: restoreOp.ID,
		Name:               "restore-locations",
		Result:             models.RestoreStepResultSuccess,
		CreatedDate:        models.PNow(),
		UpdatedDate:        models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	// Delete the export. Before the fix this returned a foreign-key violation;
	// the fix clears the restore pipeline in FK-safe order first.
	err = set.ExportRegistry.Delete(ctx, export.ID)
	c.Assert(err, qt.IsNil)

	// The export is gone.
	_, err = set.ExportRegistry.Get(ctx, export.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The referencing restore_operation is gone.
	_, err = set.RestoreOperationRegistry.Get(ctx, restoreOp.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The referencing restore_step is gone.
	_, err = set.RestoreStepRegistry.Get(ctx, restoreStep.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
