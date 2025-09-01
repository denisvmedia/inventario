package services_test

import (
	"context"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // Register file driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// newTestContext creates a context with test user for testing
func newTestContext() context.Context {
	// Create a test user with generated UUID
	testUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-" + generateTestID()},
			TenantID: "test-tenant-id",
		},
	}
	// Set UserID to self-reference
	testUser.UserID = testUser.ID

	return appctx.WithUser(context.Background(), testUser)
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
					AreaID: area.ID,
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
					AreaID: area.ID,
				})

				return commodity.ID, []string{}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := newTestContext()

			// Create temporary directory for test files
			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			// Create registry set
			factorySet := memory.NewFactorySet(); registrySet := factorySet.CreateServiceRegistrySet()
			c.Assert(registrySet, qt.IsNotNil)

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
					AreaID: area.ID,
				})
				commodity2, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity 2",
					AreaID: area.ID,
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
			ctx := newTestContext()

			// Create temporary directory for test files
			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			// Create registry set
			factorySet := memory.NewFactorySet(); registrySet := factorySet.CreateServiceRegistrySet()
			c.Assert(registrySet, qt.IsNotNil)

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
				// Get user-aware registries
				locReg, _ := registrySet.LocationRegistry.WithCurrentUser(ctx)
				areaReg, _ := registrySet.AreaRegistry.WithCurrentUser(ctx)
				comReg, _ := registrySet.CommodityRegistry.WithCurrentUser(ctx)
				fileReg, _ := registrySet.FileRegistry.WithCurrentUser(ctx)

				// Create location
				location, _ := locReg.Create(ctx, models.Location{Name: "Test Location"})

				// Create areas
				area1, _ := areaReg.Create(ctx, models.Area{Name: "Test Area 1", LocationID: location.ID})
				area2, _ := areaReg.Create(ctx, models.Area{Name: "Test Area 2", LocationID: location.ID})

				// Create commodities
				commodity1, _ := comReg.Create(ctx, models.Commodity{
					Name:   "Test Commodity 1",
					AreaID: area1.ID,
				})
				commodity2, _ := comReg.Create(ctx, models.Commodity{
					Name:   "Test Commodity 2",
					AreaID: area2.ID,
				})

				// Create linked files
				file1, _ := fileReg.Create(ctx, models.FileEntity{
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
				file2, _ := fileReg.Create(ctx, models.FileEntity{
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
				// Get user-aware registry
				locReg, _ := registrySet.LocationRegistry.WithCurrentUser(ctx)

				// Create location without areas
				location, _ := locReg.Create(ctx, models.Location{Name: "Test Location"})

				return location.ID, []string{}, []string{}, []string{}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := newTestContext()

			// Create temporary directory for test files
			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			// Create registry set
			factorySet := memory.NewFactorySet(); registrySet := factorySet.CreateServiceRegistrySet()
			c.Assert(registrySet, qt.IsNotNil)

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
			ctx := newTestContext()

			// Create temporary directory for test files
			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			// Create registry set
			factorySet := memory.NewFactorySet(); registrySet := factorySet.CreateServiceRegistrySet()
			c.Assert(registrySet, qt.IsNotNil)

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
