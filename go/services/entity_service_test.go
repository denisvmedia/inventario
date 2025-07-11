package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	_ "github.com/denisvmedia/inventario/internal/fileblob" // Register file driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func TestEntityService_DeleteCommodityRecursive(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*registry.Set) (string, []string) // returns commodityID and fileIDs
		expectError bool
	}{
		{
			name: "delete commodity with files",
			setupData: func(registrySet *registry.Set) (string, []string) {
				ctx := context.Background()

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
			setupData: func(registrySet *registry.Set) (string, []string) {
				ctx := context.Background()

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
			ctx := context.Background()

			// Create temporary directory for test files
			tempDir := c.TempDir()
			uploadLocation := "file://" + tempDir + "?create_dir=1"

			// Create registry set
			registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
			c.Assert(err, qt.IsNil)

			// Create service
			service := services.NewEntityService(registrySet, uploadLocation)

			// Setup test data
			commodityID, fileIDs := tt.setupData(registrySet)

			// Execute deletion
			err = service.DeleteCommodityRecursive(ctx, commodityID)

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
		setupData   func(*registry.Set) (string, []string, []string) // returns areaID, commodityIDs, fileIDs
		expectError bool
	}{
		{
			name: "delete area with commodities and files",
			setupData: func(registrySet *registry.Set) (string, []string, []string) {
				ctx := context.Background()

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
			setupData: func(registrySet *registry.Set) (string, []string, []string) {
				ctx := context.Background()

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
			ctx := context.Background()

			// Create temporary directory for test files
			tempDir := c.TempDir()
			uploadLocation := "file://" + tempDir + "?create_dir=1"

			// Create registry set
			registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
			c.Assert(err, qt.IsNil)

			// Create service
			service := services.NewEntityService(registrySet, uploadLocation)

			// Setup test data
			areaID, commodityIDs, fileIDs := tt.setupData(registrySet)

			// Execute deletion
			err = service.DeleteAreaRecursive(ctx, areaID)

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
		setupData   func(*registry.Set) (string, []string, []string, []string) // returns locationID, areaIDs, commodityIDs, fileIDs
		expectError bool
	}{
		{
			name: "delete location with areas, commodities and files",
			setupData: func(registrySet *registry.Set) (string, []string, []string, []string) {
				ctx := context.Background()

				// Create location
				location, _ := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Test Location"})

				// Create areas
				area1, _ := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Test Area 1", LocationID: location.ID})
				area2, _ := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Test Area 2", LocationID: location.ID})

				// Create commodities
				commodity1, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity 1",
					AreaID: area1.ID,
				})
				commodity2, _ := registrySet.CommodityRegistry.Create(ctx, models.Commodity{
					Name:   "Test Commodity 2",
					AreaID: area2.ID,
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
			setupData: func(registrySet *registry.Set) (string, []string, []string, []string) {
				ctx := context.Background()

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
			ctx := context.Background()

			// Create temporary directory for test files
			tempDir := c.TempDir()
			uploadLocation := "file://" + tempDir + "?create_dir=1"

			// Create registry set
			registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
			c.Assert(err, qt.IsNil)

			// Create service
			service := services.NewEntityService(registrySet, uploadLocation)

			// Setup test data
			locationID, areaIDs, commodityIDs, fileIDs := tt.setupData(registrySet)

			// Execute deletion
			err = service.DeleteLocationRecursive(ctx, locationID)

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
		setupData   func(*registry.Set) (string, string) // returns exportID and fileID
		expectError bool
	}{
		{
			name: "delete export with file",
			setupData: func(registrySet *registry.Set) (string, string) {
				ctx := context.Background()

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
					FileID:      file.ID,
				})

				return export.ID, file.ID
			},
			expectError: false,
		},
		{
			name: "delete export without file",
			setupData: func(registrySet *registry.Set) (string, string) {
				ctx := context.Background()

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
			ctx := context.Background()

			// Create temporary directory for test files
			tempDir := c.TempDir()
			uploadLocation := "file://" + tempDir + "?create_dir=1"

			// Create registry set
			registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
			c.Assert(err, qt.IsNil)

			// Create service
			service := services.NewEntityService(registrySet, uploadLocation)

			// Setup test data
			exportID, fileID := tt.setupData(registrySet)

			// Execute deletion
			err = service.DeleteExportWithFile(ctx, exportID)

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
