package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func TestThumbnailGenerationWorker_ProcessesJobsCorrectly(t *testing.T) {
	c := qt.New(t)

	// Create memory-based factory set for testing
	factorySet := memory.NewFactorySet()

	// Create test user and tenant
	userRegistry := factorySet.UserRegistry
	user, err := userRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	})
	c.Assert(err, qt.IsNil)

	tenantRegistry := factorySet.TenantRegistry
	_, err = tenantRegistry.Create(context.Background(), models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant"},
		Name:     "Test Tenant",
		Slug:     "test-tenant",
	})
	c.Assert(err, qt.IsNil)

	// Create test file
	fileRegistry := factorySet.FileRegistryFactory.CreateServiceRegistry()
	file, err := fileRegistry.Create(context.Background(), models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		File: &models.File{
			Path:         "test-image",
			OriginalPath: "test-image.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
		Title:       "Test Image",
		Description: "Test image for thumbnail generation",
	})
	c.Assert(err, qt.IsNil)

	// Create thumbnail generation service
	config := services.ThumbnailGenerationConfig{
		MaxConcurrentPerUser: 5,
		RateLimitPerMinute:   50,
		SlotDuration:         5 * time.Minute,
	}
	thumbnailService := services.NewThumbnailGenerationService(factorySet, "memory://", config)

	// Create a thumbnail generation job
	job, err := thumbnailService.RequestThumbnailGeneration(context.Background(), file.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(job.Status, qt.Equals, models.ThumbnailStatusPending)

	// Verify job was created
	jobRegistry := factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()
	retrievedJob, err := jobRegistry.GetJobByFileID(context.Background(), file.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedJob.Status, qt.Equals, models.ThumbnailStatusPending)

	// Get pending jobs (this is what the worker does)
	pendingJobs, err := thumbnailService.GetPendingJobs(context.Background(), 10)
	c.Assert(err, qt.IsNil)
	c.Assert(len(pendingJobs), qt.Equals, 1)
	c.Assert(pendingJobs[0].ID, qt.Equals, job.ID)

	// Test that the job processing logic works
	// Note: We can't test actual thumbnail generation without real files,
	// but we can test that the job workflow is correct
	c.Assert(pendingJobs[0].FileID, qt.Equals, file.ID)
	c.Assert(pendingJobs[0].Status, qt.Equals, models.ThumbnailStatusPending)
}

func TestThumbnailGenerationService_HandlesExistingJobs(t *testing.T) {
	c := qt.New(t)

	// Create memory-based factory set for testing
	factorySet := memory.NewFactorySet()

	// Create test user and tenant
	userRegistry := factorySet.UserRegistry
	user, err := userRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	})
	c.Assert(err, qt.IsNil)

	tenantRegistry := factorySet.TenantRegistry
	_, err = tenantRegistry.Create(context.Background(), models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant"},
		Name:     "Test Tenant",
		Slug:     "test-tenant",
	})
	c.Assert(err, qt.IsNil)

	// Create test file
	fileRegistry := factorySet.FileRegistryFactory.CreateServiceRegistry()
	file, err := fileRegistry.Create(context.Background(), models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		File: &models.File{
			Path:         "test-image",
			OriginalPath: "test-image.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
		Title:       "Test Image",
		Description: "Test image for thumbnail generation",
	})
	c.Assert(err, qt.IsNil)

	// Create thumbnail generation service
	config := services.ThumbnailGenerationConfig{
		MaxConcurrentPerUser: 5,
		RateLimitPerMinute:   50,
		SlotDuration:         5 * time.Minute,
	}
	thumbnailService := services.NewThumbnailGenerationService(factorySet, "memory://", config)

	// Request thumbnail generation twice
	job1, err := thumbnailService.RequestThumbnailGeneration(context.Background(), file.ID)
	c.Assert(err, qt.IsNil)

	job2, err := thumbnailService.RequestThumbnailGeneration(context.Background(), file.ID)
	c.Assert(err, qt.IsNil)

	// Should return the same job (no duplicate jobs created)
	c.Assert(job1.ID, qt.Equals, job2.ID)
	c.Assert(job2.Status, qt.Equals, models.ThumbnailStatusPending)

	// Verify only one job exists
	pendingJobs, err := thumbnailService.GetPendingJobs(context.Background(), 10)
	c.Assert(err, qt.IsNil)
	c.Assert(len(pendingJobs), qt.Equals, 1)
}
