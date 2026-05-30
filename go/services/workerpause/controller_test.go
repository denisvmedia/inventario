package workerpause_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services/workerpause"
)

func TestControllerRefreshOnce_NoRows_AllRunning(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	reg := memory.NewWorkerControlRegistry()
	ctrl := workerpause.NewController(reg)

	err := ctrl.RefreshOnce(ctx)
	c.Assert(err, qt.IsNil)

	for _, wt := range models.AllWorkerTypes() {
		c.Assert(ctrl.IsPaused(wt), qt.IsFalse)
	}
}

func TestControllerRefreshOnce_PauseMarksOnlyThatType(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	reg := memory.NewWorkerControlRegistry()
	_, err := reg.Pause(ctx, string(models.WorkerTypeExport), "operator@example.com", "maintenance window")
	c.Assert(err, qt.IsNil)

	ctrl := workerpause.NewController(reg)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)

	c.Assert(ctrl.IsPaused(models.WorkerTypeExport), qt.IsTrue)
	for _, wt := range models.AllWorkerTypes() {
		if wt == models.WorkerTypeExport {
			continue
		}
		c.Assert(ctrl.IsPaused(wt), qt.IsFalse)
	}
}

func TestControllerRefreshOnce_ResumeClearsPause(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	reg := memory.NewWorkerControlRegistry()
	ctrl := workerpause.NewController(reg)

	_, err := reg.Pause(ctx, string(models.WorkerTypeImport), "", "")
	c.Assert(err, qt.IsNil)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)
	c.Assert(ctrl.IsPaused(models.WorkerTypeImport), qt.IsTrue)

	_, err = reg.Resume(ctx, string(models.WorkerTypeImport))
	c.Assert(err, qt.IsNil)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)
	c.Assert(ctrl.IsPaused(models.WorkerTypeImport), qt.IsFalse)
}

func TestControllerRefreshOnce_FailSafeRetainsLastKnownState(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	fake := &flakyRegistry{
		rows: []*models.WorkerControl{
			{WorkerType: models.WorkerTypeThumbnail, Paused: true},
		},
	}
	ctrl := workerpause.NewController(fake)

	// First poll succeeds and records the paused state.
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)
	c.Assert(ctrl.IsPaused(models.WorkerTypeThumbnail), qt.IsTrue)

	// Next poll errors: the controller must return the error AND keep the
	// last-known paused state rather than flipping the worker back on.
	fake.err = errors.New("db unavailable")
	err := ctrl.RefreshOnce(ctx)
	c.Assert(err, qt.IsNotNil)
	c.Assert(ctrl.IsPaused(models.WorkerTypeThumbnail), qt.IsTrue)
}

func TestControllerIsPaused_UnknownTypeIsFalse(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	reg := memory.NewWorkerControlRegistry()
	ctrl := workerpause.NewController(reg)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)

	c.Assert(ctrl.IsPaused(models.WorkerType("not-a-real-worker")), qt.IsFalse)
}

func TestControllerRefreshOnce_IgnoresInvalidWorkerTypeRows(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	fake := &flakyRegistry{
		rows: []*models.WorkerControl{
			{WorkerType: models.WorkerType("legacy-removed-worker"), Paused: true},
			{WorkerType: models.WorkerTypeLoanReminder, Paused: true},
		},
	}
	ctrl := workerpause.NewController(fake)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)

	c.Assert(ctrl.IsPaused(models.WorkerTypeLoanReminder), qt.IsTrue)
	c.Assert(ctrl.IsPaused(models.WorkerType("legacy-removed-worker")), qt.IsFalse)
}

func TestControllerGaugeReflectsPauseState(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	reg := memory.NewWorkerControlRegistry()
	ctrl := workerpause.NewController(reg)

	_, err := reg.Pause(ctx, string(models.WorkerTypeGroupPurge), "", "")
	c.Assert(err, qt.IsNil)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)

	c.Assert(testutil.ToFloat64(workerpause.PausedGaugeFor(models.WorkerTypeGroupPurge)), qt.Equals, 1.0)
	c.Assert(testutil.ToFloat64(workerpause.PausedGaugeFor(models.WorkerTypeExport)), qt.Equals, 0.0)

	_, err = reg.Resume(ctx, string(models.WorkerTypeGroupPurge))
	c.Assert(err, qt.IsNil)
	c.Assert(ctrl.RefreshOnce(ctx), qt.IsNil)
	c.Assert(testutil.ToFloat64(workerpause.PausedGaugeFor(models.WorkerTypeGroupPurge)), qt.Equals, 0.0)
}

func TestControllerStartStop(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	reg := memory.NewWorkerControlRegistry()
	_, err := reg.Pause(ctx, string(models.WorkerTypeRestore), "", "")
	c.Assert(err, qt.IsNil)

	ctrl := workerpause.NewController(reg, workerpause.WithRefreshInterval(10*time.Millisecond))
	ctrl.Start(ctx)
	defer ctrl.Stop()

	// Start performs a synchronous RefreshOnce, so the paused state is
	// correct immediately without waiting for the ticker.
	c.Assert(ctrl.IsPaused(models.WorkerTypeRestore), qt.IsTrue)

	// Stop is idempotent.
	ctrl.Stop()
	ctrl.Stop()
}

// flakyRegistry is a tiny pauseRegistry stand-in whose List can be made to
// fail on demand, so the fail-safe and invalid-type paths can be exercised
// without a real backend.
type flakyRegistry struct {
	rows []*models.WorkerControl
	err  error
}

func (f *flakyRegistry) List(_ context.Context) ([]*models.WorkerControl, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}
