package admin

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// The worker-control trio is the CLI's half of the soft-pause surface
// (#1308) — `inventario workers pause export`, and the admin REST routes are
// a separate implementation. #2114 N3 flagged services/admin; these three
// were the last of it at 0%.
//
// The property worth the test is not the pause itself but the audit row: an
// admin action that fails and leaves no trace is exactly what an audit log
// exists to prevent, and a failed pause is the one an operator will want to
// find afterwards.

func auditRows(c *qt.C, fs *registry.FactorySet) []*models.AuditLog {
	c.Helper()

	rows, err := fs.AuditLogRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	return rows
}

func TestPauseWorker(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	control, err := svc.PauseWorker(ctx, string(models.WorkerTypeExport), "maintenance window")
	c.Assert(err, qt.IsNil)
	c.Assert(control, qt.IsNotNil)
	c.Check(control.Paused, qt.IsTrue)
	c.Assert(control.Reason, qt.IsNotNil)
	c.Check(*control.Reason, qt.Equals, "maintenance window")
	// The CLI runs with no operator session, so the row says so rather than
	// attributing the pause to whoever last used the REST surface.
	c.Assert(control.PausedBy, qt.IsNotNil)
	c.Check(*control.PausedBy, qt.Equals, "cli")

	rows := auditRows(c, fs)
	c.Assert(rows, qt.HasLen, 1)
	c.Check(rows[0].Action, qt.Equals, "admin.worker_pause")
	c.Check(rows[0].Success, qt.IsTrue)
	c.Check(rows[0].EntityID, qt.IsNotNil)
	c.Check(*rows[0].EntityID, qt.Equals, string(models.WorkerTypeExport))
}

// A typo must fail clearly rather than write a control row for a worker that
// does not exist — a paused worker nothing reads is a pause that silently
// does nothing.
func TestPauseWorker_UnknownTypeIsRefusedAndAudited(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	control, err := svc.PauseWorker(ctx, "expot", "typo")
	c.Assert(err, qt.ErrorIs, ErrUnknownWorkerType)
	c.Check(control, qt.IsNil)

	existing, listErr := fs.WorkerControlRegistry.List(ctx)
	c.Assert(listErr, qt.IsNil)
	c.Check(existing, qt.HasLen, 0)

	// The attempt is on the record even though it failed.
	rows := auditRows(c, fs)
	c.Assert(rows, qt.HasLen, 1)
	c.Check(rows[0].Action, qt.Equals, "admin.worker_pause")
	c.Check(rows[0].Success, qt.IsFalse)
	c.Assert(rows[0].ErrorMessage, qt.IsNotNil)
	c.Check(*rows[0].ErrorMessage, qt.Contains, "unknown worker type")
	// The subject is what the operator typed, not a normalized guess.
	c.Assert(rows[0].EntityID, qt.IsNotNil)
	c.Check(*rows[0].EntityID, qt.Equals, "expot")
}

func TestPauseWorker_MissingRegistryIsAConfigError(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	fs.WorkerControlRegistry = nil
	svc := &Service{factorySet: fs}

	_, err := svc.PauseWorker(ctx, string(models.WorkerTypeExport), "")
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidConfig)

	rows := auditRows(c, fs)
	c.Assert(rows, qt.HasLen, 1)
	c.Check(rows[0].Success, qt.IsFalse)
}

// Re-pausing keeps the original paused_at: the operator wants to know when
// the pause started, not when it was last restated.
func TestPauseWorker_IsIdempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	first, err := svc.PauseWorker(ctx, string(models.WorkerTypeExport), "first")
	c.Assert(err, qt.IsNil)

	second, err := svc.PauseWorker(ctx, string(models.WorkerTypeExport), "second")
	c.Assert(err, qt.IsNil)
	c.Check(second.Paused, qt.IsTrue)
	c.Assert(second.Reason, qt.IsNotNil)
	c.Check(*second.Reason, qt.Equals, "second")
	c.Check(second.PausedAt, qt.DeepEquals, first.PausedAt)

	controls, err := fs.WorkerControlRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(controls, qt.HasLen, 1)
}

func TestResumeWorker(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	_, err := svc.PauseWorker(ctx, string(models.WorkerTypeExport), "maintenance")
	c.Assert(err, qt.IsNil)

	resumed, err := svc.ResumeWorker(ctx, string(models.WorkerTypeExport))
	c.Assert(err, qt.IsNil)
	c.Assert(resumed, qt.IsNotNil)
	c.Check(resumed.Paused, qt.IsFalse)

	rows := auditRows(c, fs)
	c.Assert(rows, qt.HasLen, 2)
	c.Check(rows[1].Action, qt.Equals, "admin.worker_resume")
	c.Check(rows[1].Success, qt.IsTrue)
}

// Resuming a worker that was never paused is a no-op rather than an error:
// an operator clearing a pause that has already lapsed should not have to
// care which it was.
func TestResumeWorker_NotPausedIsANoOp(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	resumed, err := svc.ResumeWorker(ctx, string(models.WorkerTypeExport))
	c.Assert(err, qt.IsNil)
	c.Assert(resumed, qt.IsNotNil)
	c.Check(resumed.Paused, qt.IsFalse)
}

func TestResumeWorker_UnknownTypeIsRefusedAndAudited(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	_, err := svc.ResumeWorker(ctx, "expot")
	c.Assert(err, qt.ErrorIs, ErrUnknownWorkerType)

	rows := auditRows(c, fs)
	c.Assert(rows, qt.HasLen, 1)
	c.Check(rows[0].Action, qt.Equals, "admin.worker_resume")
	c.Check(rows[0].Success, qt.IsFalse)
}

func TestListWorkerControls(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := &Service{factorySet: fs}

	empty, err := svc.ListWorkerControls(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(empty, qt.HasLen, 0)

	_, err = svc.PauseWorker(ctx, string(models.WorkerTypeExport), "maintenance")
	c.Assert(err, qt.IsNil)

	controls, err := svc.ListWorkerControls(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(controls, qt.HasLen, 1)
	c.Check(controls[0].Paused, qt.IsTrue)
}

func TestListWorkerControls_MissingRegistryIsAConfigError(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	fs.WorkerControlRegistry = nil
	svc := &Service{factorySet: fs}

	_, err := svc.ListWorkerControls(context.Background())
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidConfig)
}

// The audit writer is best-effort: a deployment without an audit registry
// must still be able to pause a worker.
func TestWorkerControls_SurviveAMissingAuditRegistry(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	fs.AuditLogRegistry = nil
	svc := &Service{factorySet: fs}

	control, err := svc.PauseWorker(ctx, string(models.WorkerTypeExport), "maintenance")
	c.Assert(err, qt.IsNil)
	c.Check(control.Paused, qt.IsTrue)
}
