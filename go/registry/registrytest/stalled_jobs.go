package registrytest

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// StalledJobEnv is what RunStalledJobContract needs from a backend: a member
// of an empty group with the member's registries in it, and the service
// registries the export, import and restore workers use.
type StalledJobEnv struct {
	Ctx      context.Context
	Set      *registry.Set
	Exports  registry.ExportRegistry
	Restores registry.RestoreOperationRegistry
}

// RunStalledJobContract checks how a backend keeps track of the runs of
// exports, imports and restores, and how it recovers the runs whose worker
// stopped: a claim counts a run and records its first heartbeat, a
// heartbeat renews only a running job, and a sweep queues a dead export or
// import again until it has used its runs, then fails it, and fails a dead
// restore at once. newEnv is called once per subtest.
func RunStalledJobContract(t *testing.T, newEnv func(t *testing.T) StalledJobEnv) {
	tests := []struct {
		name string
		run  func(t *testing.T, env StalledJobEnv)
	}{
		{"a claim counts a run and records its first heartbeat", stalledJobContractClaim},
		{"a heartbeat renews only a running job", stalledJobContractHeartbeat},
		{"a dead export run is queued again until its runs are used", stalledJobContractExportRecovery},
		{"exports and imports are swept apart", stalledJobContractImportRecovery},
		{"a run without a heartbeat is dead only once its job is old", stalledJobContractNoHeartbeat},
		{"a dead restore fails at once", stalledJobContractRestoreRecovery},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newEnv(t))
		})
	}
}

// stalledNow is the time every sweep of the contract runs at.
var stalledNow = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

const stalledError = "the worker stopped"

func stalledSweep() registry.StalledJobSweep {
	return registry.StalledJobSweep{
		HeartbeatBefore: stalledNow.Add(-10 * time.Minute),
		CreatedBefore:   stalledNow.Add(-24 * time.Hour),
		Now:             stalledNow,
		MaxRuns:         3,
		ErrorMessage:    stalledError,
	}
}

// stalledExport stores an export of jobType in the env's group.
func stalledExport(c *qt.C, env StalledJobEnv, export models.Export) *models.Export {
	c.Helper()
	if export.Type == "" {
		export.Type = models.ExportTypeFullDatabase
	}
	if export.Type == models.ExportTypeImported {
		export.Imported = true
	}
	if export.CreatedDate == nil {
		export.CreatedDate = models.NewPTimestamp(stalledNow.Add(-time.Hour))
	}
	if export.Description == "" {
		export.Description = "Weekly backup"
	}
	created, err := env.Set.ExportRegistry.Create(env.Ctx, export)
	c.Assert(err, qt.IsNil)
	return created
}

func stalledRestore(c *qt.C, env StalledJobEnv, op models.RestoreOperation) *models.RestoreOperation {
	c.Helper()
	source := stalledExport(c, env, models.Export{Status: models.ExportStatusCompleted})
	op.ExportID = source.ID
	op.Description = "Restore weekly backup"
	op.Options = models.RestoreOptions{Strategy: "merge_add"}
	if op.CreatedDate == nil {
		op.CreatedDate = models.NewPTimestamp(stalledNow.Add(-time.Hour))
	}
	created, err := env.Set.RestoreOperationRegistry.Create(env.Ctx, op)
	c.Assert(err, qt.IsNil)
	return created
}

func getExport(c *qt.C, env StalledJobEnv, id string) *models.Export {
	c.Helper()
	export, err := env.Exports.Get(context.Background(), id)
	c.Assert(err, qt.IsNil)
	return export
}

func getRestore(c *qt.C, env StalledJobEnv, id string) *models.RestoreOperation {
	c.Helper()
	op, err := env.Restores.Get(context.Background(), id)
	c.Assert(err, qt.IsNil)
	return op
}

// sameTime compares a stored timestamp with t at the second, which is what
// the column keeps.
func sameTime(c *qt.C, got models.PTimestamp, want time.Time) {
	c.Helper()
	c.Assert(got, qt.IsNotNil)
	c.Assert(got.ToTime().Equal(want.Truncate(time.Second)), qt.IsTrue, qt.Commentf("got %s, want %s", *got, want))
}

func stalledJobContractClaim(t *testing.T, env StalledJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	claimedAt := stalledNow.Add(-time.Minute)

	export := stalledExport(c, env, models.Export{Status: models.ExportStatusPending, AttemptCount: 1})
	claimed, err := env.Exports.ClaimPending(ctx, export.ID, claimedAt)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	got := getExport(c, env, export.ID)
	c.Assert(got.Status, qt.Equals, models.ExportStatusInProgress)
	c.Assert(got.AttemptCount, qt.Equals, 2, qt.Commentf("the claim counts the run, on top of the earlier ones"))
	sameTime(c, got.HeartbeatAt, claimedAt)

	op := stalledRestore(c, env, models.RestoreOperation{Status: models.RestoreStatusPending})
	claimed, err = env.Restores.ClaimPending(ctx, op.ID, claimedAt)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	gotOp := getRestore(c, env, op.ID)
	c.Assert(gotOp.Status, qt.Equals, models.RestoreStatusRunning)
	sameTime(c, gotOp.HeartbeatAt, claimedAt)
}

func stalledJobContractHeartbeat(t *testing.T, env StalledJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	claimedAt := stalledNow.Add(-5 * time.Minute)

	running := stalledExport(c, env, models.Export{Status: models.ExportStatusPending})
	_, err := env.Exports.ClaimPending(ctx, running.ID, claimedAt)
	c.Assert(err, qt.IsNil)
	done := stalledExport(c, env, models.Export{Status: models.ExportStatusCompleted})

	c.Assert(env.Exports.Heartbeat(ctx, running.ID, stalledNow), qt.IsNil)
	c.Assert(env.Exports.Heartbeat(ctx, done.ID, stalledNow), qt.IsNil)
	sameTime(c, getExport(c, env, running.ID).HeartbeatAt, stalledNow)
	c.Assert(getExport(c, env, done.ID).HeartbeatAt, qt.IsNil, qt.Commentf("a finished export has no run to renew"))

	op := stalledRestore(c, env, models.RestoreOperation{Status: models.RestoreStatusPending})
	_, err = env.Restores.ClaimPending(ctx, op.ID, claimedAt)
	c.Assert(err, qt.IsNil)
	doneOp := stalledRestore(c, env, models.RestoreOperation{Status: models.RestoreStatusCompleted})

	c.Assert(env.Restores.Heartbeat(ctx, op.ID, stalledNow), qt.IsNil)
	c.Assert(env.Restores.Heartbeat(ctx, doneOp.ID, stalledNow), qt.IsNil)
	sameTime(c, getRestore(c, env, op.ID).HeartbeatAt, stalledNow)
	c.Assert(getRestore(c, env, doneOp.ID).HeartbeatAt, qt.IsNil)
}

func stalledJobContractExportRecovery(t *testing.T, env StalledJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	stale := models.NewPTimestamp(stalledNow.Add(-30 * time.Minute))

	dead := stalledExport(c, env, models.Export{Status: models.ExportStatusInProgress, AttemptCount: 1, HeartbeatAt: stale})
	alive := stalledExport(c, env, models.Export{
		Status: models.ExportStatusInProgress, AttemptCount: 1,
		HeartbeatAt: models.NewPTimestamp(stalledNow.Add(-5 * time.Minute)),
	})
	spent := stalledExport(c, env, models.Export{Status: models.ExportStatusInProgress, AttemptCount: 3, HeartbeatAt: stale})
	trashed := stalledExport(c, env, models.Export{
		Status: models.ExportStatusInProgress, AttemptCount: 1, HeartbeatAt: stale, DeletedAt: models.NewPTimestamp(stalledNow),
	})

	out, err := env.Exports.RecoverStalled(ctx, models.WorkerTypeExport, stalledSweep())
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{Requeued: 1, Failed: 1})

	got := getExport(c, env, dead.ID)
	c.Assert(got.Status, qt.Equals, models.ExportStatusPending)
	c.Assert(got.HeartbeatAt, qt.IsNil)
	c.Assert(got.AttemptCount, qt.Equals, 1, qt.Commentf("the next claim counts the next run"))

	c.Assert(getExport(c, env, alive.ID).Status, qt.Equals, models.ExportStatusInProgress)

	got = getExport(c, env, spent.ID)
	c.Assert(got.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(got.ErrorMessage, qt.Equals, stalledError)
	sameTime(c, got.CompletedDate, stalledNow)

	all, err := env.Exports.ListWithDeleted(ctx)
	c.Assert(err, qt.IsNil)
	statuses := map[string]models.ExportStatus{}
	for _, e := range all {
		statuses[e.ID] = e.Status
	}
	c.Assert(statuses[trashed.ID], qt.Equals, models.ExportStatusInProgress, qt.Commentf("an export in the trash is left alone"))

	out, err = env.Exports.RecoverStalled(ctx, models.WorkerTypeExport, stalledSweep())
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{}, qt.Commentf("a second sweep finds nothing left to do"))

	_, err = env.Exports.RecoverStalled(ctx, models.WorkerTypeRestore, stalledSweep())
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
}

func stalledJobContractImportRecovery(t *testing.T, env StalledJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	stale := models.NewPTimestamp(stalledNow.Add(-30 * time.Minute))

	export := stalledExport(c, env, models.Export{Status: models.ExportStatusInProgress, AttemptCount: 3, HeartbeatAt: stale})
	imported := stalledExport(c, env, models.Export{
		Type: models.ExportTypeImported, Status: models.ExportStatusInProgress, AttemptCount: 3, HeartbeatAt: stale,
	})

	out, err := env.Exports.RecoverStalled(ctx, models.WorkerTypeImport, stalledSweep())
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{Failed: 1})
	c.Assert(getExport(c, env, imported.ID).Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(getExport(c, env, export.ID).Status, qt.Equals, models.ExportStatusInProgress,
		qt.Commentf("the import sweep leaves exports to the export worker"))

	out, err = env.Exports.RecoverStalled(ctx, models.WorkerTypeExport, stalledSweep())
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{Failed: 1})
	c.Assert(getExport(c, env, export.ID).Status, qt.Equals, models.ExportStatusFailed)
}

func stalledJobContractNoHeartbeat(t *testing.T, env StalledJobEnv) {
	c := qt.New(t)
	ctx := context.Background()

	// Backends stamp the creation time of an export themselves, so the
	// sweep moves its cutoff around the rows instead.
	export := stalledExport(c, env, models.Export{Status: models.ExportStatusInProgress})
	op := stalledRestore(c, env, models.RestoreOperation{Status: models.RestoreStatusRunning})
	created := getExport(c, env, export.ID).CreatedDate.ToTime()

	young := stalledSweep()
	young.CreatedBefore = created.Add(-time.Hour)
	out, err := env.Exports.RecoverStalled(ctx, models.WorkerTypeExport, young)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{}, qt.Commentf("a version without heartbeats may still be running it"))
	failed, err := env.Restores.FailStalled(ctx, young)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.Equals, 0)

	old := stalledSweep()
	old.CreatedBefore = created.Add(time.Hour)
	out, err = env.Exports.RecoverStalled(ctx, models.WorkerTypeExport, old)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, registry.StalledJobs{Requeued: 1})
	c.Assert(getExport(c, env, export.ID).Status, qt.Equals, models.ExportStatusPending)
	failed, err = env.Restores.FailStalled(ctx, old)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.Equals, 1)
	c.Assert(getRestore(c, env, op.ID).Status, qt.Equals, models.RestoreStatusFailed)
}

func stalledJobContractRestoreRecovery(t *testing.T, env StalledJobEnv) {
	c := qt.New(t)
	ctx := context.Background()

	dead := stalledRestore(c, env, models.RestoreOperation{
		Status: models.RestoreStatusRunning, HeartbeatAt: models.NewPTimestamp(stalledNow.Add(-30 * time.Minute)),
	})
	alive := stalledRestore(c, env, models.RestoreOperation{
		Status: models.RestoreStatusRunning, HeartbeatAt: models.NewPTimestamp(stalledNow.Add(-5 * time.Minute)),
	})
	pending := stalledRestore(c, env, models.RestoreOperation{Status: models.RestoreStatusPending})

	failed, err := env.Restores.FailStalled(ctx, stalledSweep())
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.Equals, 1)

	got := getRestore(c, env, dead.ID)
	c.Assert(got.Status, qt.Equals, models.RestoreStatusFailed, qt.Commentf("a restore may have written part of its backup"))
	c.Assert(got.ErrorMessage, qt.Equals, stalledError)
	c.Assert(got.HeartbeatAt, qt.IsNil)
	sameTime(c, got.CompletedDate, stalledNow)
	c.Assert(getRestore(c, env, alive.ID).Status, qt.Equals, models.RestoreStatusRunning)
	c.Assert(getRestore(c, env, pending.ID).Status, qt.Equals, models.RestoreStatusPending)

	failed, err = env.Restores.FailStalled(ctx, stalledSweep())
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.Equals, 0)
}
