package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// failedJobFixture is a memory backend with one group and helpers that
// store failed jobs in it.
type failedJobFixture struct {
	fs  *registry.FactorySet
	ctx context.Context
	set *registry.Set
}

func newFailedJobFixture(c *qt.C) *failedJobFixture {
	fs := memory.NewFactorySet()
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "user-1"}, TenantID: "tenant-1"},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "group-1"}, TenantID: "tenant-1"},
		Slug:                "group-1",
		GroupCurrency:       "USD",
	})
	set, err := fs.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	return &failedJobFixture{fs: fs, ctx: ctx, set: set}
}

func (f *failedJobFixture) export(c *qt.C, typ models.ExportType, failedAt time.Time) string {
	c.Helper()
	e, err := f.set.ExportRegistry.Create(f.ctx, models.Export{
		Type:          typ,
		Imported:      typ == models.ExportTypeImported,
		Status:        models.ExportStatusFailed,
		Description:   "backup " + failedAt.Format(time.Kitchen),
		ErrorMessage:  "disk full",
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-time.Minute)),
		CompletedDate: models.NewPTimestamp(failedAt),
	})
	c.Assert(err, qt.IsNil)
	return e.ID
}

func (f *failedJobFixture) restore(c *qt.C, failedAt time.Time) string {
	c.Helper()
	source, err := f.set.ExportRegistry.Create(f.ctx, models.Export{
		Type:   models.ExportTypeFullDatabase,
		Status: models.ExportStatusCompleted,
	})
	c.Assert(err, qt.IsNil)
	op, err := f.set.RestoreOperationRegistry.Create(f.ctx, models.RestoreOperation{
		ExportID:      source.ID,
		Description:   "restore",
		Status:        models.RestoreStatusFailed,
		Options:       models.RestoreOptions{Strategy: "merge_add"},
		ErrorMessage:  "conflict",
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-time.Minute)),
		CompletedDate: models.NewPTimestamp(failedAt),
	})
	c.Assert(err, qt.IsNil)
	return op.ID
}

func TestFailedJobService_ListMergesTypesNewestFirst(t *testing.T) {
	c := qt.New(t)
	f := newFailedJobFixture(c)
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	oldExport := f.export(c, models.ExportTypeFullDatabase, base)
	restore := f.restore(c, base.Add(time.Hour))
	newExport := f.export(c, models.ExportTypeFullDatabase, base.Add(2*time.Hour))
	imported := f.export(c, models.ExportTypeImported, base.Add(3*time.Hour))
	svc := services.NewFailedJobService(f.fs.FailedJobRegistry)

	page, err := svc.List(context.Background(), services.FailedJobQuery{})
	c.Assert(err, qt.IsNil)
	c.Assert(page.Total, qt.Equals, 4)
	c.Assert(jobIDs(page.Jobs), qt.DeepEquals, []string{imported, newExport, restore, oldExport})
	c.Assert(page.Jobs[0].Type, qt.Equals, models.WorkerTypeImport)
	c.Assert(page.Jobs[2].Type, qt.Equals, models.WorkerTypeRestore)

	page, err = svc.List(context.Background(), services.FailedJobQuery{Offset: 1, Limit: 2})
	c.Assert(err, qt.IsNil)
	c.Assert(page.Total, qt.Equals, 4, qt.Commentf("the total counts every page"))
	c.Assert(jobIDs(page.Jobs), qt.DeepEquals, []string{newExport, restore})

	page, err = svc.List(context.Background(), services.FailedJobQuery{Offset: 10, Limit: 2})
	c.Assert(err, qt.IsNil)
	c.Assert(page.Jobs, qt.HasLen, 0)
	c.Assert(page.Total, qt.Equals, 4)

	page, err = svc.List(context.Background(), services.FailedJobQuery{Type: models.WorkerTypeExport})
	c.Assert(err, qt.IsNil)
	c.Assert(jobIDs(page.Jobs), qt.DeepEquals, []string{newExport, oldExport})
	c.Assert(page.Total, qt.Equals, 2)

	page, err = svc.List(context.Background(), services.FailedJobQuery{
		Scope: registry.FailedJobScope{TenantID: "tenant-1", GroupID: "another-group"},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(page.Jobs, qt.HasLen, 0)
	c.Assert(page.Total, qt.Equals, 0)

	_, err = svc.List(context.Background(), services.FailedJobQuery{Type: models.WorkerTypeWeeklyDigest})
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
}

func TestFailedJobService_Retry(t *testing.T) {
	c := qt.New(t)
	f := newFailedJobFixture(c)
	id := f.export(c, models.ExportTypeFullDatabase, time.Now().Add(-time.Hour))
	restore := f.restore(c, time.Now().Add(-time.Hour))
	svc := services.NewFailedJobService(f.fs.FailedJobRegistry)
	scope := registry.FailedJobScope{TenantID: "tenant-1", GroupID: "group-1"}

	job, err := svc.Retry(context.Background(), models.WorkerTypeExport, scope, id)
	c.Assert(err, qt.IsNil)
	c.Assert(job.ID, qt.Equals, id)
	c.Assert(job.Error, qt.Equals, "disk full", qt.Commentf("the job comes back as it failed, for the audit"))
	export, err := f.set.ExportRegistry.Get(f.ctx, id)
	c.Assert(err, qt.IsNil)
	c.Assert(export.Status, qt.Equals, models.ExportStatusPending)

	_, err = svc.Retry(context.Background(), models.WorkerTypeExport, scope, id)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	job, err = svc.Retry(context.Background(), models.WorkerTypeRestore, scope, restore)
	c.Assert(err, qt.ErrorIs, registry.ErrFailedJobNotRetryable)
	c.Assert(job, qt.IsNotNil)

	_, err = svc.Retry(context.Background(), "nope", scope, id)
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
}

func TestFailedJobService_DiscardAndDepths(t *testing.T) {
	c := qt.New(t)
	f := newFailedJobFixture(c)
	first := f.export(c, models.ExportTypeFullDatabase, time.Now().Add(-2*time.Hour))
	f.export(c, models.ExportTypeFullDatabase, time.Now().Add(-time.Hour))
	restore := f.restore(c, time.Now().Add(-time.Hour))
	svc := services.NewFailedJobService(f.fs.FailedJobRegistry)

	depths, err := svc.Depths(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(depths, qt.DeepEquals, map[string]int{
		"export": 2, "import": 0, "restore": 1, "thumbnail": 0, "intake": 0, "currency-migration": 0,
		"personal-data-export": 0,
	})

	job, err := svc.Discard(context.Background(), models.WorkerTypeRestore, registry.FailedJobScope{}, restore)
	c.Assert(err, qt.IsNil)
	c.Assert(job.ID, qt.Equals, restore)
	_, err = svc.Discard(context.Background(), models.WorkerTypeExport, registry.FailedJobScope{}, first)
	c.Assert(err, qt.IsNil)

	depths, err = svc.Depths(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(depths["export"], qt.Equals, 1)
	c.Assert(depths["restore"], qt.Equals, 0)

	op, err := f.set.RestoreOperationRegistry.Get(f.ctx, restore)
	c.Assert(err, qt.IsNil)
	c.Assert(op.Status, qt.Equals, models.RestoreStatusFailed, qt.Commentf("discard keeps the row"))

	_, err = svc.Discard(context.Background(), models.WorkerTypeRestore, registry.FailedJobScope{}, restore)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func jobIDs(jobs []*models.FailedJob) []string {
	ids := make([]string, 0, len(jobs))
	for _, j := range jobs {
		ids = append(ids, j.ID)
	}
	return ids
}

// personalExport stores a personal data export of the fixture's user whose
// build failed at failedAt.
func (f *failedJobFixture) personalExport(c *qt.C, failedAt time.Time) string {
	c.Helper()
	reg := f.fs.PersonalDataExportRegistry
	created, _, err := reg.CreateUnlessRequestedSince(f.ctx, models.PersonalDataExport{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: "tenant-1", UserID: "user-1"},
		CreatedAt:               failedAt.Add(-time.Minute),
		ExpiresAt:               failedAt.Add(models.PersonalDataExportRetention),
	}, failedAt)
	c.Assert(err, qt.IsNil)
	created.Status = models.PersonalDataExportStatusFailed
	created.ErrorMessage = "the account is not active"
	created.CompletedAt = new(failedAt)
	_, err = reg.Update(f.ctx, *created)
	c.Assert(err, qt.IsNil)
	return created.ID
}

// The installation-wide list holds personal data exports; a group's list,
// and every call in a group, does not know their type.
func TestFailedJobService_PersonalDataExportsAreNotInAGroup(t *testing.T) {
	c := qt.New(t)
	f := newFailedJobFixture(c)
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	export := f.export(c, models.ExportTypeFullDatabase, base)
	personal := f.personalExport(c, base.Add(time.Hour))
	svc := services.NewFailedJobService(f.fs.FailedJobRegistry)
	group := registry.FailedJobScope{TenantID: "tenant-1", GroupID: "group-1"}

	page, err := svc.List(context.Background(), services.FailedJobQuery{})
	c.Assert(err, qt.IsNil)
	c.Assert(jobIDs(page.Jobs), qt.DeepEquals, []string{personal, export})

	page, err = svc.List(context.Background(), services.FailedJobQuery{Scope: group})
	c.Assert(err, qt.IsNil)
	c.Assert(jobIDs(page.Jobs), qt.DeepEquals, []string{export})
	c.Assert(page.Total, qt.Equals, 1)

	_, err = svc.List(context.Background(), services.FailedJobQuery{Type: models.FailedJobTypePersonalDataExport, Scope: group})
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
	_, err = svc.Retry(context.Background(), models.FailedJobTypePersonalDataExport, group, personal)
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)

	depths, err := svc.Depths(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(depths["personal-data-export"], qt.Equals, 1)
}
