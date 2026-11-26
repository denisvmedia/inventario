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
		ErrorMessage:  "conflicting ids",
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

func TestRefusesMemoryDatabase(t *testing.T) {
	c := qt.New(t)
	_, err := run(c, "memory://", "failed", "list")
	c.Assert(err, qt.ErrorMatches, "failed-job commands are not supported for memory databases.*")
}
