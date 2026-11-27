package failed_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/cmd/inventario/jobs"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

const testDSN = "postgres://test:test@localhost/test"

// fixture registers a memory store under the postgres name, so the admin
// service the commands open shares it, and stores a failed export and a
// failed restore in one tenant.
type fixture struct {
	fs      *registry.FactorySet
	ctx     context.Context
	set     *registry.Set
	export  string
	restore string
}

func newFixture(c *qt.C) *fixture {
	newFn, _ := memory.NewMemoryRegistrySet()
	fs, err := newFn(registry.Config(testDSN))
	c.Assert(err, qt.IsNil)
	registry.Register("postgres", func(registry.Config) (*registry.FactorySet, error) { return fs, nil })
	c.Cleanup(func() { registry.Unregister("postgres") })

	tenant, err := fs.TenantRegistry.Create(context.Background(), models.Tenant{
		Name: "Acme", Slug: "acme", Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "user-1"}, TenantID: tenant.ID},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "group-1"}, TenantID: tenant.ID},
		Slug:                "group-1",
		GroupCurrency:       "USD",
	})
	set, err := fs.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)

	failedAt := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	export, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:          models.ExportTypeFullDatabase,
		Status:        models.ExportStatusFailed,
		Description:   "Weekly backup",
		ErrorMessage:  "disk full\non the volume",
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-time.Minute)),
		CompletedDate: models.NewPTimestamp(failedAt),
	})
	c.Assert(err, qt.IsNil)
	source, err := set.ExportRegistry.Create(ctx, models.Export{Type: models.ExportTypeFullDatabase, Status: models.ExportStatusCompleted})
	c.Assert(err, qt.IsNil)
	restore, err := set.RestoreOperationRegistry.Create(ctx, models.RestoreOperation{
		ExportID:      source.ID,
		Description:   "Restore",
		Status:        models.RestoreStatusFailed,
		Options:       models.RestoreOptions{Strategy: "merge_add"},
		ErrorMessage:  "conflicting\x1b[2J ids",
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-time.Hour)),
		CompletedDate: models.NewPTimestamp(failedAt.Add(-time.Minute)),
	})
	c.Assert(err, qt.IsNil)
	return &fixture{fs: fs, ctx: ctx, set: set, export: export.ID, restore: restore.ID}
}

func run(c *qt.C, dsn string, args ...string) (string, error) {
	c.Helper()
	cmd := jobs.New(&shared.DatabaseConfig{DBDSN: dsn})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func (f *fixture) auditRows(c *qt.C, action string) []*models.AuditLog {
	c.Helper()
	rows, err := f.fs.AuditLogRegistry.ListByAction(context.Background(), action)
	c.Assert(err, qt.IsNil)
	return rows
}

func TestList(t *testing.T) {
	c := qt.New(t)
	f := newFixture(c)

	out, err := run(c, testDSN, "failed", "list")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "TYPE")
	c.Assert(out, qt.Contains, f.export)
	c.Assert(out, qt.Contains, f.restore)
	c.Assert(out, qt.Contains, "disk full on the volume", qt.Commentf("an error stays on its row"))
	c.Assert(out, qt.Contains, "conflicting [2J ids")
	c.Assert(out, qt.Not(qt.Contains), "\x1b", qt.Commentf("no control character reaches the terminal"))

	out, err = run(c, testDSN, "failed", "list", "--type", "restore", "--tenant", "acme")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, f.restore)
	c.Assert(out, qt.Not(qt.Contains), f.export)

	out, err = run(c, testDSN, "failed", "list", "--limit", "1")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Showing 1 of 2 failed jobs")

	_, err = run(c, testDSN, "failed", "list", "--type", "weekly-digest")
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)

	_, err = run(c, testDSN, "failed", "list", "--tenant", "nobody")
	c.Assert(err, qt.ErrorMatches, ".*tenant 'nobody' not found.*")
}

func TestRetry(t *testing.T) {
	c := qt.New(t)
	f := newFixture(c)

	out, err := run(c, testDSN, "failed", "retry", "--type", "export", f.export)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Retried export job "+f.export)
	export, err := f.set.ExportRegistry.Get(f.ctx, f.export)
	c.Assert(err, qt.IsNil)
	c.Assert(export.Status, qt.Equals, models.ExportStatusPending)

	_, err = run(c, testDSN, "failed", "retry", "--type", "restore", f.restore)
	c.Assert(err, qt.ErrorIs, registry.ErrFailedJobNotRetryable)

	rows := f.auditRows(c, "admin.failed_job_retry")
	c.Assert(rows, qt.HasLen, 2)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(rows[0].UserID, qt.IsNil, qt.Commentf("the CLI has no actor"))
	c.Assert(*rows[0].EntityID, qt.Equals, f.export)
	c.Assert(rows[1].Success, qt.IsFalse)

	_, err = run(c, testDSN, "failed", "retry", f.export)
	c.Assert(err, qt.ErrorMatches, "--type is required")
	_, err = run(c, testDSN, "failed", "retry", "--type", "nope", f.export)
	c.Assert(err, qt.ErrorMatches, `unknown job type "nope".*`)
	rows = f.auditRows(c, "admin.failed_job_retry")
	c.Assert(rows, qt.HasLen, 3, qt.Commentf("an unknown type is refused and audited"))
	c.Assert(rows[2].Success, qt.IsFalse)
	c.Assert(*rows[2].EntityType, qt.Equals, "nope")
	c.Assert(*rows[2].EntityID, qt.Equals, f.export)
}

func TestDiscard(t *testing.T) {
	c := qt.New(t)
	f := newFixture(c)

	out, err := run(c, testDSN, "failed", "discard", "--type", "restore", f.restore)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Discarded restore job "+f.restore)
	restore, err := f.set.RestoreOperationRegistry.Get(f.ctx, f.restore)
	c.Assert(err, qt.IsNil)
	c.Assert(restore.Status, qt.Equals, models.RestoreStatusFailed)
	c.Assert(restore.DiscardedAt, qt.IsNotNil)
	c.Assert(f.auditRows(c, "admin.failed_job_discard"), qt.HasLen, 1)

	_, err = run(c, testDSN, "failed", "discard", "--type", "restore", f.restore)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	out, err = run(c, testDSN, "failed", "list", "--type", "restore")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "No failed jobs.")
}

// The fixture's jobs failed on 2026-11-20, the export at 10:00 and the
// restore a minute before.
func TestDiscardBefore(t *testing.T) {
	c := qt.New(t)
	f := newFixture(c)

	out, err := run(c, testDSN, "failed", "discard", "--failed-before", "2026-11-21", "--dry-run")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, "Would discard 2 failed jobs that failed before 2026-11-21T00:00:00Z: export 1, restore 1. Nothing was discarded.\n")
	c.Assert(f.auditRows(c, "admin.failed_job_bulk_discard"), qt.HasLen, 0, qt.Commentf("a dry run is not audited"))

	out, err = run(c, testDSN, "failed", "list", "--failed-before", "2026-11-20T10:00:00Z")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, f.restore)
	c.Assert(out, qt.Not(qt.Contains), f.export, qt.Commentf("the export failed at 10:00, not before"))

	out, err = run(c, testDSN, "failed", "discard", "--type", "export", "--tenant", "acme", "--failed-before", "2026-11-21")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, "Discarded 1 failed job that failed before 2026-11-21T00:00:00Z: export 1.\n")
	export, err := f.set.ExportRegistry.Get(f.ctx, f.export)
	c.Assert(err, qt.IsNil)
	c.Assert(export.DiscardedAt, qt.IsNotNil)

	rows := f.auditRows(c, "admin.failed_job_bulk_discard")
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(rows[0].UserID, qt.IsNil, qt.Commentf("the CLI has no actor"))
	c.Assert(*rows[0].EntityType, qt.Equals, "export")
	c.Assert(rows[0].TenantID, qt.IsNotNil)
	c.Assert([]byte(rows[0].UserAgent), checkers.JSONPathEquals("$.discarded"), float64(1))
	c.Assert([]byte(rows[0].UserAgent), checkers.JSONPathEquals("$.failed_before"), "2026-11-21T00:00:00Z")
	c.Assert([]byte(rows[0].UserAgent), checkers.JSONPathEquals("$.tenant"), "acme")

	out, err = run(c, testDSN, "failed", "discard", "--failed-before", "2026-11-21")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, "Discarded 1 failed job that failed before 2026-11-21T00:00:00Z: restore 1.\n")
	out, err = run(c, testDSN, "failed", "discard", "--failed-before", "2026-11-21")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Equals, "No failed jobs failed before 2026-11-21T00:00:00Z.\n")
	c.Assert(f.auditRows(c, "admin.failed_job_bulk_discard"), qt.HasLen, 3)
}

func TestDiscardBefore_RefusesABadFilter(t *testing.T) {
	c := qt.New(t)
	f := newFixture(c)

	_, err := run(c, testDSN, "failed", "discard")
	c.Assert(err, qt.ErrorMatches, "give a job id, or --failed-before .*")
	_, err = run(c, testDSN, "failed", "discard", "--failed-before", "yesterday")
	c.Assert(err, qt.ErrorMatches, `invalid --failed-before "yesterday".*`)
	_, err = run(c, testDSN, "failed", "discard", "--type", "restore", "--dry-run", f.restore)
	c.Assert(err, qt.ErrorMatches, "--failed-before, --tenant and --dry-run discard in bulk.*")
	c.Assert(f.auditRows(c, "admin.failed_job_bulk_discard"), qt.HasLen, 0, qt.Commentf("refused before the service runs"))

	_, err = run(c, testDSN, "failed", "discard", "--failed-before", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidInput)
	_, err = run(c, testDSN, "failed", "discard", "--type", "nope", "--failed-before", "2026-11-21")
	c.Assert(err, qt.ErrorMatches, `unknown job type "nope".*`)
	_, err = run(c, testDSN, "failed", "discard", "--tenant", "nobody", "--failed-before", "2026-11-21")
	c.Assert(err, qt.ErrorMatches, ".*tenant 'nobody' not found.*")

	rows := f.auditRows(c, "admin.failed_job_bulk_discard")
	c.Assert(rows, qt.HasLen, 3, qt.Commentf("a discard the service refuses is audited"))
	for _, row := range rows {
		c.Assert(row.Success, qt.IsFalse)
	}
	out, err := run(c, testDSN, "failed", "list")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, f.export)
	c.Assert(out, qt.Contains, f.restore)
}

func TestRefusesMemoryDatabase(t *testing.T) {
	c := qt.New(t)
	_, err := run(c, "memory://", "failed", "list")
	c.Assert(err, qt.ErrorMatches, "failed-job commands are not supported for memory databases.*")
}
