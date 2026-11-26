package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// thumbnailFixture is one image file of a group, whose blob does not exist,
// so every thumbnail run of it fails.
type thumbnailFixture struct {
	fs      *registry.FactorySet
	service *services.ThumbnailGenerationService
	file    *models.FileEntity
	reqCtx  context.Context
}

func newThumbnailFixture(c *qt.C) *thumbnailFixture {
	c.Helper()
	ctx := context.Background()
	fs := memory.NewFactorySet()
	user := must.Must(fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "test-tenant"},
		Email:               "thumbs@example.com",
		Name:                "Thumbs",
		IsActive:            true,
	}))
	group := must.Must(fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Home",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
	}))
	file := must.Must(fs.FileRegistryFactory.CreateServiceRegistry().Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: user.TenantID, GroupID: group.ID, CreatedByUserID: user.ID},
		File:                     &models.File{Path: "broken", OriginalPath: "broken.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
		Title:                    "Broken photo",
	}))
	return &thumbnailFixture{
		fs: fs,
		service: services.NewThumbnailGenerationService(fs, "memory://", services.ThumbnailGenerationConfig{
			MaxConcurrentPerUser: 5,
			RateLimitPerMinute:   50,
			SlotDuration:         5 * time.Minute,
		}),
		file:   file,
		reqCtx: appctx.WithUser(ctx, user),
	}
}

func (f *thumbnailFixture) fail(c *qt.C, job *models.ThumbnailGenerationJob) {
	c.Helper()
	reg := f.fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()
	c.Assert(reg.UpdateJobStatus(context.Background(), job.ID, models.ThumbnailStatusFailed, "unsupported image"), qt.IsNil)
}

// A file whose thumbnail failed for good keeps that one failed job while
// nobody retries or discards it: viewing the file again starts no new job
// (#1307). A discarded failure lets the next view try again.
func TestThumbnailGenerationService_FailedJobAwaitsOperator(t *testing.T) {
	c := qt.New(t)
	f := newThumbnailFixture(c)

	first := must.Must(f.service.RequestThumbnailGeneration(f.reqCtx, f.file.ID))
	f.fail(c, first)

	again := must.Must(f.service.RequestThumbnailGeneration(f.reqCtx, f.file.ID))
	c.Assert(again.ID, qt.Equals, first.ID)
	c.Assert(again.Status, qt.Equals, models.ThumbnailStatusFailed)
	jobs := must.Must(f.fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry().ListByFileID(context.Background(), f.file.ID))
	c.Assert(jobs, qt.HasLen, 1)

	c.Assert(f.fs.FailedJobRegistry.Discard(context.Background(), models.WorkerTypeThumbnail, registry.FailedJobScope{}, first.ID, time.Now()), qt.IsNil)
	next := must.Must(f.service.RequestThumbnailGeneration(f.reqCtx, f.file.ID))
	c.Assert(next.ID, qt.Not(qt.Equals), first.ID)
	c.Assert(next.Status, qt.Equals, models.ThumbnailStatusPending)

	// When the new job fails too, it is the file's failed job again.
	f.fail(c, next)
	failed := must.Must(f.fs.FailedJobRegistry.ListFailed(context.Background(), models.WorkerTypeThumbnail, registry.FailedJobScope{}, 0))
	c.Assert(failed, qt.HasLen, 1)
	c.Assert(failed[0].ID, qt.Equals, next.ID)
}

// Only the run that uses up the job's attempts marks it failed for good,
// and the dead-letter counter goes up once for it (#1307).
func TestThumbnailGenerationService_FailureIsCountedOnce(t *testing.T) {
	c := qt.New(t)
	f := newThumbnailFixture(c)
	job := must.Must(f.service.RequestThumbnailGeneration(f.reqCtx, f.file.ID))
	reg := f.fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	counter := metrics.JobsDeadLetteredFor(string(models.WorkerTypeThumbnail))
	before := testutil.ToFloat64(counter)

	for attempt := 1; attempt <= job.MaxAttempts; attempt++ {
		c.Assert(f.service.ProcessThumbnailGeneration(context.Background(), job.ID), qt.IsNotNil)
		current := must.Must(reg.Get(context.Background(), job.ID))
		c.Assert(current.AttemptCount, qt.Equals, attempt)
		want, counted := models.ThumbnailStatusPending, float64(0)
		if attempt == job.MaxAttempts {
			want, counted = models.ThumbnailStatusFailed, 1
		}
		c.Assert(current.Status, qt.Equals, want, qt.Commentf("attempt %d", attempt))
		c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, counted, qt.Commentf("attempt %d", attempt))
	}
}
