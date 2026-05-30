package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// TestWorkerControlRegistry_Pause_Creates_Postgres verifies the first
// Pause writes a paused row carrying the operator, reason, and a pause
// timestamp. worker_control is NOT tenant-scoped, so no seeded
// tenant/user/group fixtures are required — a clean factory set suffices.
func TestWorkerControlRegistry_Pause_Creates_Postgres(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()
	reg := fs.WorkerControlRegistry

	wc, err := reg.Pause(ctx, string(models.WorkerTypeExport), "operator@example.com", "maintenance window")
	c.Assert(err, qt.IsNil)
	c.Assert(wc.WorkerType, qt.Equals, models.WorkerTypeExport)
	c.Assert(wc.Paused, qt.IsTrue)
	c.Assert(wc.PausedBy, qt.IsNotNil)
	c.Assert(*wc.PausedBy, qt.Equals, "operator@example.com")
	c.Assert(wc.Reason, qt.IsNotNil)
	c.Assert(*wc.Reason, qt.Equals, "maintenance window")
	c.Assert(wc.PausedAt, qt.IsNotNil)
	c.Assert(wc.PausedAt.IsZero(), qt.IsFalse)
	c.Assert(wc.UpdatedAt.IsZero(), qt.IsFalse)

	controls, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 1)
}

// TestWorkerControlRegistry_Pause_IdempotentPreservesPausedAt_Postgres
// verifies a second Pause updates paused_by/reason but PRESERVES the
// original paused_at via the CASE expression — the pause time must
// reflect when the worker first stopped, not the latest note edit.
func TestWorkerControlRegistry_Pause_IdempotentPreservesPausedAt_Postgres(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()
	reg := fs.WorkerControlRegistry

	first, err := reg.Pause(ctx, string(models.WorkerTypeThumbnail), "alice", "first reason")
	c.Assert(err, qt.IsNil)
	c.Assert(first.PausedAt, qt.IsNotNil)
	originalPausedAt := *first.PausedAt

	second, err := reg.Pause(ctx, string(models.WorkerTypeThumbnail), "bob", "second reason")
	c.Assert(err, qt.IsNil)
	c.Assert(second.Paused, qt.IsTrue)
	// paused_at preserved...
	c.Assert(second.PausedAt, qt.IsNotNil)
	c.Assert(second.PausedAt.Equal(originalPausedAt), qt.IsTrue)
	// ...but by/reason updated.
	c.Assert(*second.PausedBy, qt.Equals, "bob")
	c.Assert(*second.Reason, qt.Equals, "second reason")

	// Still a single row (ON CONFLICT collapsed onto it).
	controls, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 1)
}

// TestWorkerControlRegistry_Pause_EmptyByAndReasonStoredAsNull_Postgres
// verifies the CLI pause path (no operator session, no reason) lands NULL
// columns rather than empty-string sentinels.
func TestWorkerControlRegistry_Pause_EmptyByAndReasonStoredAsNull_Postgres(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()
	reg := fs.WorkerControlRegistry

	wc, err := reg.Pause(ctx, string(models.WorkerTypeImport), "", "")
	c.Assert(err, qt.IsNil)
	c.Assert(wc.Paused, qt.IsTrue)
	c.Assert(wc.PausedBy, qt.IsNil)
	c.Assert(wc.Reason, qt.IsNil)
}

// TestWorkerControlRegistry_Resume_Clears_Postgres verifies Resume flips
// paused to false and clears paused_at/paused_by/reason.
func TestWorkerControlRegistry_Resume_Clears_Postgres(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()
	reg := fs.WorkerControlRegistry

	_, err := reg.Pause(ctx, string(models.WorkerTypeGroupPurge), "alice", "cleanup")
	c.Assert(err, qt.IsNil)

	resumed, err := reg.Resume(ctx, string(models.WorkerTypeGroupPurge))
	c.Assert(err, qt.IsNil)
	c.Assert(resumed.WorkerType, qt.Equals, models.WorkerTypeGroupPurge)
	c.Assert(resumed.Paused, qt.IsFalse)
	c.Assert(resumed.PausedBy, qt.IsNil)
	c.Assert(resumed.PausedAt, qt.IsNil)
	c.Assert(resumed.Reason, qt.IsNil)
}

// TestWorkerControlRegistry_Resume_Absent_Postgres verifies resuming a
// worker that was never paused is a no-op: it returns a synthetic
// not-paused state and does NOT insert a row.
func TestWorkerControlRegistry_Resume_Absent_Postgres(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()
	reg := fs.WorkerControlRegistry

	resumed, err := reg.Resume(ctx, string(models.WorkerTypeLoanReminder))
	c.Assert(err, qt.IsNil)
	c.Assert(resumed.WorkerType, qt.Equals, models.WorkerTypeLoanReminder)
	c.Assert(resumed.Paused, qt.IsFalse)

	controls, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 0)
}

// TestWorkerControlRegistry_List_OrderedByWorkerType_Postgres verifies
// List comes back sorted by worker_type so the CLI/admin render is
// deterministic.
func TestWorkerControlRegistry_List_OrderedByWorkerType_Postgres(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()
	reg := fs.WorkerControlRegistry

	_, err := reg.Pause(ctx, string(models.WorkerTypeThumbnail), "", "")
	c.Assert(err, qt.IsNil)
	_, err = reg.Pause(ctx, string(models.WorkerTypeExport), "", "")
	c.Assert(err, qt.IsNil)
	_, err = reg.Pause(ctx, string(models.WorkerTypeImport), "", "")
	c.Assert(err, qt.IsNil)

	controls, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 3)

	types := make([]models.WorkerType, len(controls))
	for i, wc := range controls {
		types[i] = wc.WorkerType
	}
	c.Assert(types, qt.DeepEquals, []models.WorkerType{
		models.WorkerTypeExport,
		models.WorkerTypeImport,
		models.WorkerTypeThumbnail,
	})
}

// TestWorkerControlRegistry_RejectsUnknownType_Postgres verifies the
// registry-level defence-in-depth validation (#1308 review fix): Pause
// and Resume reject a worker type that is not in the canonical set,
// before touching the table.
func TestWorkerControlRegistry_RejectsUnknownType_Postgres(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()
	reg := fs.WorkerControlRegistry

	_, err := reg.Pause(ctx, "not-a-worker", "alice", "x")
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidInput)

	_, err = reg.Resume(ctx, "not-a-worker")
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidInput)

	// No row was written for the rejected type.
	controls, err := reg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 0)
}
