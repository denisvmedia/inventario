package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestExportRegistry_Memory_DeleteClearsRestorePipeline is the memory-backend
// mirror of the #2118 regression (see registry/postgres/exports_test.go). The
// postgres backend MUST clear restore_operations/restore_steps before deleting
// an export because restore_operations.export_id is a NOT NULL NO ACTION FK;
// memory has no FK enforcement, but the two backends have to behave
// identically — so memory ExportRegistry.Delete clears the same restore
// pipeline. Without the fix the in-memory store was left with restore
// operations orphaned to a deleted export, and a memory-backed export-delete
// test would pass whether or not the postgres cleanup existed. This seeds an
// export + restore_operation + restore_step, deletes the export, and asserts
// all three are gone.
func TestExportRegistry_Memory_DeleteClearsRestorePipeline(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()

	userID := "test-user-123"
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	testUser.SetPassword("Password123")

	serviceRegistrySet := factorySet.CreateServiceRegistrySet()
	createdUser, err := serviceRegistrySet.UserRegistry.Create(context.Background(), testUser)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), createdUser)
	rs := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Seed an export.
	export, err := rs.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "Export with restore pipeline",
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	// Seed a restore_operation referencing the export.
	restoreOp, err := rs.RestoreOperationRegistry.Create(ctx, models.RestoreOperation{
		ExportID:    export.ID,
		Description: "Restore from export",
		Status:      models.RestoreStatusCompleted,
		Options: models.RestoreOptions{
			Strategy: "full_replace",
		},
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	// Seed a restore_step referencing the operation.
	restoreStep, err := rs.RestoreStepRegistry.Create(ctx, models.RestoreStep{
		RestoreOperationID: restoreOp.ID,
		Name:               "restore-locations",
		Result:             models.RestoreStepResultSuccess,
		CreatedDate:        models.PNow(),
		UpdatedDate:        models.PNow(),
	})
	c.Assert(err, qt.IsNil)

	// Sanity: the operation is discoverable by export before deletion.
	opsBefore, err := rs.RestoreOperationRegistry.ListByExport(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(opsBefore, qt.HasLen, 1)

	// Delete the export — must clear the restore pipeline first.
	err = rs.ExportRegistry.Delete(ctx, export.ID)
	c.Assert(err, qt.IsNil)

	// The export is gone.
	_, err = rs.ExportRegistry.Get(ctx, export.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The referencing restore_operation is gone (no orphan left behind).
	_, err = rs.RestoreOperationRegistry.Get(ctx, restoreOp.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	opsAfter, err := rs.RestoreOperationRegistry.ListByExport(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(opsAfter, qt.HasLen, 0)

	// The referencing restore_step is gone (cascaded via the operation delete).
	_, err = rs.RestoreStepRegistry.Get(ctx, restoreStep.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
