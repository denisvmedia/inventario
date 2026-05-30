package memory_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

// newWorkerControlRegistryForTest builds a registry with deterministic
// nowFn/uuidFn so timestamp- and id-sensitive assertions are stable.
// nowFn advances by one second per call so re-pause vs original pause
// times are distinguishable; uuidFn hands out predictable ids.
func newWorkerControlRegistryForTest(t *testing.T) (*memory.WorkerControlRegistry, *time.Time) {
	t.Helper()

	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := base
	var idSeq int
	r := memory.NewWorkerControlRegistryForTesting(
		func() time.Time {
			cur := clock
			clock = clock.Add(time.Second)
			return cur
		},
		func() string {
			idSeq++
			return "id-" + strconv.Itoa(idSeq)
		},
	)
	return r, &base
}

// TestWorkerControlRegistry_List_Empty verifies a fresh registry reports
// no control rows — the caller treats this as "every worker running".
func TestWorkerControlRegistry_List_Empty(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewWorkerControlRegistry()

	controls, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 0)
}

// TestWorkerControlRegistry_Pause_Creates verifies the first Pause writes
// a paused row carrying the operator, reason, and a pause timestamp.
func TestWorkerControlRegistry_Pause_Creates(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r, base := newWorkerControlRegistryForTest(t)

	wc, err := r.Pause(ctx, string(models.WorkerTypeExport), "operator@example.com", "maintenance window")
	c.Assert(err, qt.IsNil)
	c.Assert(wc.WorkerType, qt.Equals, models.WorkerTypeExport)
	c.Assert(wc.Paused, qt.IsTrue)
	c.Assert(wc.PausedBy, qt.IsNotNil)
	c.Assert(*wc.PausedBy, qt.Equals, "operator@example.com")
	c.Assert(wc.Reason, qt.IsNotNil)
	c.Assert(*wc.Reason, qt.Equals, "maintenance window")
	c.Assert(wc.PausedAt, qt.IsNotNil)
	c.Assert(wc.PausedAt.Equal(*base), qt.IsTrue)

	controls, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 1)
}

// TestWorkerControlRegistry_Pause_EmptyByAndReasonStoredAsNull verifies
// that the CLI pause path (no operator session, no reason) lands NULLs
// rather than empty-string sentinels.
func TestWorkerControlRegistry_Pause_EmptyByAndReasonStoredAsNull(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r, _ := newWorkerControlRegistryForTest(t)

	wc, err := r.Pause(ctx, string(models.WorkerTypeImport), "", "")
	c.Assert(err, qt.IsNil)
	c.Assert(wc.Paused, qt.IsTrue)
	c.Assert(wc.PausedBy, qt.IsNil)
	c.Assert(wc.Reason, qt.IsNil)
}

// TestWorkerControlRegistry_Pause_IdempotentPreservesPausedAt verifies a
// second Pause updates paused_by/reason but keeps the ORIGINAL paused_at,
// so the timestamp reflects when the worker first stopped.
func TestWorkerControlRegistry_Pause_IdempotentPreservesPausedAt(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r, base := newWorkerControlRegistryForTest(t)

	first, err := r.Pause(ctx, string(models.WorkerTypeThumbnail), "alice", "first reason")
	c.Assert(err, qt.IsNil)
	originalPausedAt := *first.PausedAt
	c.Assert(originalPausedAt.Equal(*base), qt.IsTrue)

	second, err := r.Pause(ctx, string(models.WorkerTypeThumbnail), "bob", "second reason")
	c.Assert(err, qt.IsNil)
	c.Assert(second.Paused, qt.IsTrue)
	// paused_at unchanged...
	c.Assert(second.PausedAt, qt.IsNotNil)
	c.Assert(second.PausedAt.Equal(originalPausedAt), qt.IsTrue)
	// ...but by/reason updated...
	c.Assert(*second.PausedBy, qt.Equals, "bob")
	c.Assert(*second.Reason, qt.Equals, "second reason")
	// ...and updated_at advanced past the original pause time.
	c.Assert(second.UpdatedAt.After(originalPausedAt), qt.IsTrue)

	// Still a single row.
	controls, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 1)
}

// TestWorkerControlRegistry_Resume_Clears verifies resume flips paused to
// false and clears paused_at/paused_by/reason.
func TestWorkerControlRegistry_Resume_Clears(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r, _ := newWorkerControlRegistryForTest(t)

	_, err := r.Pause(ctx, string(models.WorkerTypeGroupPurge), "alice", "cleanup")
	c.Assert(err, qt.IsNil)

	resumed, err := r.Resume(ctx, string(models.WorkerTypeGroupPurge))
	c.Assert(err, qt.IsNil)
	c.Assert(resumed.WorkerType, qt.Equals, models.WorkerTypeGroupPurge)
	c.Assert(resumed.Paused, qt.IsFalse)
	c.Assert(resumed.PausedBy, qt.IsNil)
	c.Assert(resumed.PausedAt, qt.IsNil)
	c.Assert(resumed.Reason, qt.IsNil)
}

// TestWorkerControlRegistry_Resume_Absent verifies resuming a worker that
// was never paused is a no-op returning a synthetic not-paused state and
// does NOT create a row.
func TestWorkerControlRegistry_Resume_Absent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewWorkerControlRegistry()

	resumed, err := r.Resume(ctx, string(models.WorkerTypeLoanReminder))
	c.Assert(err, qt.IsNil)
	c.Assert(resumed.WorkerType, qt.Equals, models.WorkerTypeLoanReminder)
	c.Assert(resumed.Paused, qt.IsFalse)

	controls, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 0)
}

// TestWorkerControlRegistry_List_OrderedByWorkerType verifies List comes
// back sorted by worker_type so the CLI/admin render is deterministic.
func TestWorkerControlRegistry_List_OrderedByWorkerType(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r, _ := newWorkerControlRegistryForTest(t)

	_, err := r.Pause(ctx, string(models.WorkerTypeThumbnail), "", "")
	c.Assert(err, qt.IsNil)
	_, err = r.Pause(ctx, string(models.WorkerTypeExport), "", "")
	c.Assert(err, qt.IsNil)
	_, err = r.Pause(ctx, string(models.WorkerTypeImport), "", "")
	c.Assert(err, qt.IsNil)

	controls, err := r.List(ctx)
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
