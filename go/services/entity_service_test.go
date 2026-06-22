package services_test

import (
	"context"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // Register file driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// uploadLocationForTempDir builds a file:// upload-location URL for the given
// temp dir, matching the OS-specific scheme the rest of the package uses.
func uploadLocationForTempDir(tempDir string) string {
	if runtime.GOOS == "windows" {
		return "file:///" + tempDir + "?create_dir=1"
	}
	return "file://" + tempDir + "?create_dir=1"
}

// newTestContext creates a context with test user for testing
func newTestContext(factorySet *registry.FactorySet) context.Context {
	// Create a test user with generated UUID
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-" + generateTestID()},
			TenantID: "test-tenant-id",
		},
		Email: "test@example.com",
		Name:  "Test User",
	}
	// Register the user in the system
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u := must.Must(userReg.Create(context.Background(), testUser))

	return appctx.WithUser(context.Background(), u)
}

// generateTestID generates a simple test ID
func generateTestID() string {
	return "12345678-1234-1234-1234-123456789012" // Fixed UUID for consistent testing
}

func TestEntityService_DeleteCommodityRecursive(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(context.Context, *registry.Set) (string, []string) // returns commodityID and fileIDs
		expectError bool
	}{
		{
			name: "delete commodity with files",
			setupData: func(ctx context.Context, registrySet *registry.Set) (string, []string) {
				// Create location and area
				location, _ := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Test Location"})
				area, _ := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Test Area", LocationID: location.ID})

				// Create commodity
				commodity, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity",
					AreaID: new(area.ID),
				})

				// Create linked files
				file1, _ := registrySet.FileRegistry.Create(ctx, models.FileEntity{
					LinkedEntityType: "commodity",
					LinkedEntityID:   commodity.ID,
					LinkedEntityMeta: "images",
					File: &models.File{
						Path:         "test1",
						OriginalPath: "test1.jpg",
						Ext:          ".jpg",
						MIMEType:     "image/jpeg",
					},
				})
				file2, _ := registrySet.FileRegistry.Create(ctx, models.FileEntity{
					LinkedEntityType: "commodity",
					LinkedEntityID:   commodity.ID,
					LinkedEntityMeta: "manuals",
					File: &models.File{
						Path:         "test2",
						OriginalPath: "test2.pdf",
						Ext:          ".pdf",
						MIMEType:     "application/pdf",
					},
				})

				return commodity.ID, []string{file1.ID, file2.ID}
			},
			expectError: false,
		},
		{
			name: "delete commodity without files",
			setupData: func(ctx context.Context, registrySet *registry.Set) (string, []string) {
				// Create location and area
				location, _ := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Test Location"})
				area, _ := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Test Area", LocationID: location.ID})

				// Create commodity without files
				commodity, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity",
					AreaID: new(area.ID),
				})

				return commodity.ID, []string{}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create temporary directory for test files
			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			// Create factory set and user context
			factorySet := memory.NewFactorySet()
			ctx := newTestContext(factorySet)
			registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

			// Create service
			service := services.NewEntityService(factorySet, uploadLocation)

			// Setup test data
			commodityID, fileIDs := tt.setupData(ctx, registrySet)

			// Execute deletion
			err := service.DeleteCommodityRecursive(ctx, commodityID)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				return
			}

			c.Assert(err, qt.IsNil)

			// Verify commodity is deleted
			_, err = registrySet.CommodityRegistry.Get(ctx, commodityID)
			c.Assert(err, qt.Equals, registry.ErrNotFound)

			// Verify all files are deleted
			for _, fileID := range fileIDs {
				_, err = registrySet.FileRegistry.Get(ctx, fileID)
				c.Assert(err, qt.Equals, registry.ErrNotFound)
			}
		})
	}
}

func TestEntityService_DeleteAreaRecursive(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(context.Context, *registry.Set) (string, []string, []string) // returns areaID, commodityIDs, fileIDs
		expectError bool
	}{
		{
			name: "delete area with commodities and files",
			setupData: func(ctx context.Context, registrySet *registry.Set) (string, []string, []string) {
				// Create location and area
				location, _ := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Test Location"})
				area, _ := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Test Area", LocationID: location.ID})

				// Create commodities
				commodity1, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity 1",
					AreaID: new(area.ID),
				})
				commodity2, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity 2",
					AreaID: new(area.ID),
				})

				// Create linked files
				file1, _ := registrySet.FileRegistry.Create(ctx, models.FileEntity{
					LinkedEntityType: "commodity",
					LinkedEntityID:   commodity1.ID,
					LinkedEntityMeta: "images",
					File: &models.File{
						Path:         "test1",
						OriginalPath: "test1.jpg",
						Ext:          ".jpg",
						MIMEType:     "image/jpeg",
					},
				})
				file2, _ := registrySet.FileRegistry.Create(ctx, models.FileEntity{
					LinkedEntityType: "commodity",
					LinkedEntityID:   commodity2.ID,
					LinkedEntityMeta: "manuals",
					File: &models.File{
						Path:         "test2",
						OriginalPath: "test2.pdf",
						Ext:          ".pdf",
						MIMEType:     "application/pdf",
					},
				})

				return area.ID, []string{commodity1.ID, commodity2.ID}, []string{file1.ID, file2.ID}
			},
			expectError: false,
		},
		{
			name: "delete area without commodities",
			setupData: func(ctx context.Context, registrySet *registry.Set) (string, []string, []string) {
				// Create location and area
				location, _ := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Test Location"})
				area, _ := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Test Area", LocationID: location.ID})

				return area.ID, []string{}, []string{}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create temporary directory for test files
			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			// Create factory set and user context
			factorySet := memory.NewFactorySet()
			ctx := newTestContext(factorySet)
			registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

			// Create service
			service := services.NewEntityService(factorySet, uploadLocation)

			// Setup test data
			areaID, commodityIDs, fileIDs := tt.setupData(ctx, registrySet)

			// Execute deletion
			err := service.DeleteAreaRecursive(ctx, areaID)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				return
			}

			c.Assert(err, qt.IsNil)

			// Verify area is deleted
			_, err = registrySet.AreaRegistry.Get(ctx, areaID)
			c.Assert(err, qt.Equals, registry.ErrNotFound)

			// Verify all commodities are deleted
			for _, commodityID := range commodityIDs {
				_, err = registrySet.CommodityRegistry.Get(ctx, commodityID)
				c.Assert(err, qt.Equals, registry.ErrNotFound)
			}

			// Verify all files are deleted
			for _, fileID := range fileIDs {
				_, err = registrySet.FileRegistry.Get(ctx, fileID)
				c.Assert(err, qt.Equals, registry.ErrNotFound)
			}
		})
	}
}

func TestEntityService_DeleteLocationRecursive(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(context.Context, *registry.Set) (string, []string, []string, []string) // returns locationID, areaIDs, commodityIDs, fileIDs
		expectError bool
	}{
		{
			name: "delete location with areas, commodities and files",
			setupData: func(ctx context.Context, registrySet *registry.Set) (string, []string, []string, []string) {
				// Create location
				location, _ := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Test Location"})

				// Create areas
				area1, _ := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Test Area 1", LocationID: location.ID})
				area2, _ := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Test Area 2", LocationID: location.ID})

				// Create commodities
				commodity1, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity 1",
					AreaID: new(area1.ID),
				})
				commodity2, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity 2",
					AreaID: new(area2.ID),
				})

				// Create linked files
				file1, _ := registrySet.FileRegistry.Create(ctx, models.FileEntity{
					LinkedEntityType: "commodity",
					LinkedEntityID:   commodity1.ID,
					LinkedEntityMeta: "images",
					File: &models.File{
						Path:         "test1",
						OriginalPath: "test1.jpg",
						Ext:          ".jpg",
						MIMEType:     "image/jpeg",
					},
				})
				file2, _ := registrySet.FileRegistry.Create(ctx, models.FileEntity{
					LinkedEntityType: "commodity",
					LinkedEntityID:   commodity2.ID,
					LinkedEntityMeta: "manuals",
					File: &models.File{
						Path:         "test2",
						OriginalPath: "test2.pdf",
						Ext:          ".pdf",
						MIMEType:     "application/pdf",
					},
				})

				return location.ID, []string{area1.ID, area2.ID}, []string{commodity1.ID, commodity2.ID}, []string{file1.ID, file2.ID}
			},
			expectError: false,
		},
		{
			name: "delete location without areas",
			setupData: func(ctx context.Context, registrySet *registry.Set) (string, []string, []string, []string) {
				// Create location without areas
				location, _ := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Test Location"})

				return location.ID, []string{}, []string{}, []string{}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create temporary directory for test files
			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			// Create factory set and user context
			factorySet := memory.NewFactorySet()
			ctx := newTestContext(factorySet)
			registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

			// Create service
			service := services.NewEntityService(factorySet, uploadLocation)

			// Setup test data
			locationID, areaIDs, commodityIDs, fileIDs := tt.setupData(ctx, registrySet)

			// Execute deletion
			err := service.DeleteLocationRecursive(ctx, locationID)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				return
			}

			c.Assert(err, qt.IsNil)

			// Verify location is deleted
			_, err = registrySet.LocationRegistry.Get(ctx, locationID)
			c.Assert(err, qt.Equals, registry.ErrNotFound)

			// Verify all areas are deleted
			for _, areaID := range areaIDs {
				_, err = registrySet.AreaRegistry.Get(ctx, areaID)
				c.Assert(err, qt.Equals, registry.ErrNotFound)
			}

			// Verify all commodities are deleted
			for _, commodityID := range commodityIDs {
				_, err = registrySet.CommodityRegistry.Get(ctx, commodityID)
				c.Assert(err, qt.Equals, registry.ErrNotFound)
			}

			// Verify all files are deleted
			for _, fileID := range fileIDs {
				_, err = registrySet.FileRegistry.Get(ctx, fileID)
				c.Assert(err, qt.Equals, registry.ErrNotFound)
			}
		})
	}
}

func TestEntityService_DeleteExportWithFile(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(context.Context, *registry.Set) (string, string) // returns exportID and fileID
		expectError bool
	}{
		{
			name: "delete export with file",
			setupData: func(ctx context.Context, registrySet *registry.Set) (string, string) {
				// Create file
				file, _ := registrySet.FileRegistry.Create(ctx, models.FileEntity{
					LinkedEntityType: "export",
					LinkedEntityMeta: "xml-1.0",
					File: &models.File{
						Path:         "export",
						OriginalPath: "export.xml",
						Ext:          ".xml",
						MIMEType:     "application/xml",
					},
				})

				// Create export with file
				export, _ := registrySet.ExportRegistry.Create(ctx, models.Export{
					Description: "Test Export",
					FileID:      &file.ID,
				})

				return export.ID, file.ID
			},
			expectError: false,
		},
		{
			name: "delete export without file",
			setupData: func(ctx context.Context, registrySet *registry.Set) (string, string) {
				// Create export without file
				export, _ := registrySet.ExportRegistry.Create(ctx, models.Export{
					Description: "Test Export",
				})

				return export.ID, ""
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create temporary directory for test files
			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			// Create factory set and user context
			factorySet := memory.NewFactorySet()
			ctx := newTestContext(factorySet)
			registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

			// Create service
			service := services.NewEntityService(factorySet, uploadLocation)

			// Setup test data
			exportID, fileID := tt.setupData(ctx, registrySet)

			// Execute deletion
			err := service.DeleteExportWithFile(ctx, exportID)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				return
			}

			c.Assert(err, qt.IsNil)

			// Verify export is deleted
			_, err = registrySet.ExportRegistry.Get(ctx, exportID)
			c.Assert(err, qt.Equals, registry.ErrNotFound)

			// Verify file is deleted if it existed
			if fileID != "" {
				_, err = registrySet.FileRegistry.Get(ctx, fileID)
				c.Assert(err, qt.Equals, registry.ErrNotFound)
			}
		})
	}
}

// TestEntityService_DeleteExport_CleansPendingImportSourceBlob is the #2121
// regression: an imported export carries its uploaded source `.inb` blob under
// FilePath until import processing promotes it into a FileEntity (FileID). While
// the export is still pending (FileID == nil) that blob has no owning file row,
// so neither the single-file delete nor the group/tenant file sweep (both
// iterate `files` rows) would ever clean it up. Deleting such an export must
// best-effort remove its source blob so it doesn't leak permanently.
func TestEntityService_DeleteExport_CleansPendingImportSourceBlob(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	// Write the uploaded source blob a pending imported export would point at.
	sourceKey := "t/test-tenant/restores/uploaded-backup.inb"
	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()
	c.Assert(b.WriteAll(ctx, sourceKey, []byte("signed-inb-bytes"), nil), qt.IsNil)
	c.Assert(must.Must(b.Exists(ctx, sourceKey)), qt.IsTrue)

	// A pending imported export: FilePath set, FileID still nil.
	export := must.Must(registrySet.ExportRegistry.Create(ctx, models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Pending import",
		FilePath:    sourceKey,
		Imported:    true,
	}))

	c.Assert(service.DeleteExportWithFile(ctx, export.ID), qt.IsNil)

	// The export row is gone…
	_, err := registrySet.ExportRegistry.Get(ctx, export.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// …and its orphaned source blob was cleaned up, not leaked.
	c.Assert(must.Must(b.Exists(ctx, sourceKey)), qt.IsFalse,
		qt.Commentf("pending imported-export source blob %s must not leak", sourceKey))
}

// TestDeleteFileWithPhysical_DeletesThumbnailJob asserts the #2117 cleanup
// order: deleting a file also removes the thumbnail-generation job that
// references it and the concurrency slot that references that job, so the
// NO ACTION FKs (slots -> jobs -> files) never block the file row delete.
func TestDeleteFileWithPhysical_DeletesThumbnailJob(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewFileService(factorySet, uploadLocation)

	// Write the physical blob.
	testFilePath := "thumb-source.jpg"
	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()
	err := b.WriteAll(ctx, testFilePath, []byte("image bytes"), nil)
	c.Assert(err, qt.IsNil)

	// Create the file row.
	createdFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		Type: models.FileTypeImage,
		File: &models.File{
			Path:         "thumb-source",
			OriginalPath: testFilePath,
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	// Create a thumbnail-generation job for the file and a concurrency slot
	// for that job.
	createdJob := must.Must(registrySet.ThumbnailGenerationJobRegistry.Create(ctx, models.ThumbnailGenerationJob{
		FileID:      createdFile.ID,
		Status:      models.ThumbnailStatusPending,
		MaxAttempts: 3,
	}))
	createdSlot := must.Must(registrySet.UserConcurrencySlotRegistry.Create(ctx, models.UserConcurrencySlot{
		JobID:  createdJob.ID,
		Status: models.SlotStatusActive,
	}))

	// Delete the file.
	err = service.DeleteFileWithPhysical(ctx, createdFile.ID)
	c.Assert(err, qt.IsNil)

	// The file row is gone.
	_, err = registrySet.FileRegistry.Get(ctx, createdFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The thumbnail job is gone.
	_, err = registrySet.ThumbnailGenerationJobRegistry.Get(ctx, createdJob.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The concurrency slot is gone.
	_, err = registrySet.UserConcurrencySlotRegistry.Get(ctx, createdSlot.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The physical blob is gone (best-effort cleanup ran after the row delete).
	exists := must.Must(b.Exists(ctx, testFilePath))
	c.Assert(exists, qt.IsFalse)
}

// TestDeleteFileWithPhysical_DeletesAllJobsAndSlots covers the multi-job-per-file
// case: idx_thumbnail_jobs_file_id is NOT unique, so a file can own more than
// one job (e.g. a failed job plus a retry). Deleting the file must clear EVERY
// job's concurrency slots before dropping the jobs, otherwise the second job's
// slot dangles and (on postgres) FK-fails the file delete. Here the file owns
// two jobs, each with an active slot; after the delete both jobs and both slots
// are gone.
func TestDeleteFileWithPhysical_DeletesAllJobsAndSlots(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewFileService(factorySet, uploadLocation)

	// Write the physical blob.
	testFilePath := "thumb-source-multi.jpg"
	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()
	c.Assert(b.WriteAll(ctx, testFilePath, []byte("image bytes"), nil), qt.IsNil)

	// Create the file row.
	createdFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		Type: models.FileTypeImage,
		File: &models.File{
			Path:         "thumb-source-multi",
			OriginalPath: testFilePath,
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	// Two jobs for the same file (a failed job plus a retry), each with a slot.
	failedJob := must.Must(registrySet.ThumbnailGenerationJobRegistry.Create(ctx, models.ThumbnailGenerationJob{
		FileID:      createdFile.ID,
		Status:      models.ThumbnailStatusFailed,
		MaxAttempts: 3,
	}))
	retryJob := must.Must(registrySet.ThumbnailGenerationJobRegistry.Create(ctx, models.ThumbnailGenerationJob{
		FileID:      createdFile.ID,
		Status:      models.ThumbnailStatusPending,
		MaxAttempts: 3,
	}))
	failedSlot := must.Must(registrySet.UserConcurrencySlotRegistry.Create(ctx, models.UserConcurrencySlot{
		JobID:  failedJob.ID,
		Status: models.SlotStatusActive,
	}))
	retrySlot := must.Must(registrySet.UserConcurrencySlotRegistry.Create(ctx, models.UserConcurrencySlot{
		JobID:  retryJob.ID,
		Status: models.SlotStatusActive,
	}))

	// Delete the file.
	c.Assert(service.DeleteFileWithPhysical(ctx, createdFile.ID), qt.IsNil)

	// The file row is gone.
	_, err := registrySet.FileRegistry.Get(ctx, createdFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Both jobs are gone.
	_, err = registrySet.ThumbnailGenerationJobRegistry.Get(ctx, failedJob.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = registrySet.ThumbnailGenerationJobRegistry.Get(ctx, retryJob.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Both slots are gone — including the second job's slot, which the old
	// single-job path would have left dangling.
	_, err = registrySet.UserConcurrencySlotRegistry.Get(ctx, failedSlot.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = registrySet.UserConcurrencySlotRegistry.Get(ctx, retrySlot.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// TestDeleteCommodityRecursive_RowFirst asserts the #2120 happy path: the
// commodity and its linked files (rows + blobs) are all gone after the
// row-first delete.
func TestDeleteCommodityRecursive_RowFirst(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID}))
	commodity := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:   "Commodity",
		AreaID: new(area.ID),
	}))

	// Two physical blobs + their linked file rows.
	b := must.Must(blob.OpenBucket(ctx, uploadLocation))
	defer b.Close()

	filePaths := []string{"com-1.jpg", "com-2.pdf"}
	mimeTypes := []string{"image/jpeg", "application/pdf"}
	var fileIDs []string
	for i, p := range filePaths {
		c.Assert(b.WriteAll(ctx, p, []byte("bytes"), nil), qt.IsNil)
		file := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
			LinkedEntityType: "commodity",
			LinkedEntityID:   commodity.ID,
			LinkedEntityMeta: "images",
			File: &models.File{
				Path:         p,
				OriginalPath: p,
				Ext:          ".x",
				MIMEType:     mimeTypes[i],
			},
		}))
		fileIDs = append(fileIDs, file.ID)
	}

	err := service.DeleteCommodityRecursive(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)

	// Commodity row is gone.
	_, err = registrySet.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Linked file rows + blobs are gone.
	for i, fileID := range fileIDs {
		_, err = registrySet.FileRegistry.Get(ctx, fileID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

		exists := must.Must(b.Exists(ctx, filePaths[i]))
		c.Assert(exists, qt.IsFalse)
	}
}

// TestDeleteAreaRecursive_DeletesAttachedFiles asserts #2119: files attached
// directly to an area (not via a commodity) are removed when the area is
// recursively deleted.
func TestDeleteAreaRecursive_DeletesAttachedFiles(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID}))

	areaFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		LinkedEntityType: "area",
		LinkedEntityID:   area.ID,
		LinkedEntityMeta: "images",
		File: &models.File{
			Path:         "area-doc",
			OriginalPath: "area-doc.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	err := service.DeleteAreaRecursive(ctx, area.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.AreaRegistry.Get(ctx, area.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = registrySet.FileRegistry.Get(ctx, areaFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// TestDeleteLocationRecursive_DeletesAttachedFiles asserts #2119: files
// attached directly to a location are removed when the location is recursively
// deleted.
func TestDeleteLocationRecursive_DeletesAttachedFiles(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))

	locationFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		LinkedEntityType: "location",
		LinkedEntityID:   location.ID,
		LinkedEntityMeta: "images",
		File: &models.File{
			Path:         "loc-doc",
			OriginalPath: "loc-doc.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	err := service.DeleteLocationRecursive(ctx, location.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.LocationRegistry.Get(ctx, location.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = registrySet.FileRegistry.Get(ctx, locationFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// TestDeleteArea_DeletesLinkedFiles asserts #2119: the non-recursive DeleteArea
// removes an EMPTY area together with the files attached directly to it (DB
// rows + blob), so they don't orphan.
func TestDeleteArea_DeletesLinkedFiles(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID}))

	areaFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		LinkedEntityType: "area",
		LinkedEntityID:   area.ID,
		LinkedEntityMeta: "images",
		File: &models.File{
			Path:         "area-doc",
			OriginalPath: "area-doc.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	err := service.DeleteArea(ctx, area.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.AreaRegistry.Get(ctx, area.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = registrySet.FileRegistry.Get(ctx, areaFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	linked := must.Must(registrySet.FileRegistry.ListByLinkedEntity(ctx, "area", area.ID))
	c.Assert(linked, qt.HasLen, 0)
}

// TestDeleteArea_NonEmptyRejected asserts #2119: DeleteArea is non-recursive —
// an area that still holds a commodity is rejected with ErrCannotDelete and
// nothing is removed.
func TestDeleteArea_NonEmptyRejected(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID}))
	commodity := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:   "Commodity",
		AreaID: new(area.ID),
	}))

	err := service.DeleteArea(ctx, area.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrCannotDelete)

	// The area and its commodity survive.
	c.Assert(must.Must(registrySet.AreaRegistry.Get(ctx, area.ID)), qt.IsNotNil)
	c.Assert(must.Must(registrySet.CommodityRegistry.Get(ctx, commodity.ID)), qt.IsNotNil)
}

// TestDeleteLocation_DeletesLinkedFiles asserts #2119: the non-recursive
// DeleteLocation removes an EMPTY location together with the files attached
// directly to it.
func TestDeleteLocation_DeletesLinkedFiles(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))

	locationFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		LinkedEntityType: "location",
		LinkedEntityID:   location.ID,
		LinkedEntityMeta: "images",
		File: &models.File{
			Path:         "loc-doc",
			OriginalPath: "loc-doc.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	err := service.DeleteLocation(ctx, location.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.LocationRegistry.Get(ctx, location.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = registrySet.FileRegistry.Get(ctx, locationFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	linked := must.Must(registrySet.FileRegistry.ListByLinkedEntity(ctx, "location", location.ID))
	c.Assert(linked, qt.HasLen, 0)
}

// TestDeleteLocation_NonEmptyRejected asserts #2119: DeleteLocation is
// non-recursive — a location that still holds an area is rejected with
// ErrCannotDelete and nothing is removed.
func TestDeleteLocation_NonEmptyRejected(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID}))

	err := service.DeleteLocation(ctx, location.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrCannotDelete)

	// The location and its area survive.
	c.Assert(must.Must(registrySet.LocationRegistry.Get(ctx, location.ID)), qt.IsNotNil)
	c.Assert(must.Must(registrySet.AreaRegistry.Get(ctx, area.ID)), qt.IsNotNil)
}

// TestUnlinkAndDeleteArea_KeepsCommodities asserts #2137: the "unlink" strategy
// removes a non-empty area (and the files attached directly to it) while
// keeping its commodities — left area-less (AreaID == nil) rather than deleted.
func TestUnlinkAndDeleteArea_KeepsCommodities(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area", LocationID: location.ID}))
	commodity1 := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:   "Commodity 1",
		AreaID: new(area.ID),
	}))
	commodity2 := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:   "Commodity 2",
		AreaID: new(area.ID),
	}))

	areaFile := must.Must(registrySet.FileRegistry.Create(ctx, models.FileEntity{
		LinkedEntityType: "area",
		LinkedEntityID:   area.ID,
		LinkedEntityMeta: "images",
		File: &models.File{
			Path:         "area-doc",
			OriginalPath: "area-doc.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	err := service.UnlinkAndDeleteArea(ctx, area.ID)
	c.Assert(err, qt.IsNil)

	// The area and the file attached directly to it are gone.
	_, err = registrySet.AreaRegistry.Get(ctx, area.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = registrySet.FileRegistry.Get(ctx, areaFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Both commodities survive, now area-less.
	got1 := must.Must(registrySet.CommodityRegistry.Get(ctx, commodity1.ID))
	c.Assert(got1.AreaID, qt.IsNil)
	got2 := must.Must(registrySet.CommodityRegistry.Get(ctx, commodity2.ID))
	c.Assert(got2.AreaID, qt.IsNil)
}

// TestUnlinkAndDeleteLocation_KeepsCommodities asserts #2137: the "unlink"
// strategy removes a non-empty location and all its areas while keeping the
// commodities filed under those areas — left area-less (AreaID == nil).
func TestUnlinkAndDeleteLocation_KeepsCommodities(t *testing.T) {
	c := qt.New(t)

	tempDir := c.TempDir()
	uploadLocation := uploadLocationForTempDir(tempDir)

	factorySet := memory.NewFactorySet()
	ctx := newTestContext(factorySet)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	service := services.NewEntityService(factorySet, uploadLocation)

	location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Loc"}))
	area1 := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area 1", LocationID: location.ID}))
	area2 := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Area 2", LocationID: location.ID}))
	commodity1 := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:   "Commodity 1",
		AreaID: new(area1.ID),
	}))
	commodity2 := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:   "Commodity 2",
		AreaID: new(area2.ID),
	}))

	err := service.UnlinkAndDeleteLocation(ctx, location.ID)
	c.Assert(err, qt.IsNil)

	// The location and both its areas are gone.
	_, err = registrySet.LocationRegistry.Get(ctx, location.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = registrySet.AreaRegistry.Get(ctx, area1.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = registrySet.AreaRegistry.Get(ctx, area2.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Both commodities survive, now area-less.
	got1 := must.Must(registrySet.CommodityRegistry.Get(ctx, commodity1.ID))
	c.Assert(got1.AreaID, qt.IsNil)
	got2 := must.Must(registrySet.CommodityRegistry.Get(ctx, commodity2.ID))
	c.Assert(got2.AreaID, qt.IsNil)
}
