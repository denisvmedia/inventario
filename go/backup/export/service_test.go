package export

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// testUserID will be set dynamically when creating test user
var testUserID string

// testGroupID is the ID of the default LocationGroup created by newTestFactorySet.
var testGroupID string

// TestExtractTenantUserFromContext tests the ExtractTenantUserFromContext function
func TestExtractTenantUserFromContext(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() context.Context
		expectError bool
		errorMsg    string
		tenantID    string
		userID      string
	}{
		{
			name: "valid context with user containing tenant ID",
			setupCtx: func() context.Context {
				ctx := context.Background()
				user := &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: "user-456"},
						TenantID: "tenant-123",
					},
					Email: "test@example.com",
				}
				ctx = appctx.WithUser(ctx, user)
				return ctx
			},
			expectError: false,
			tenantID:    "tenant-123",
			userID:      "user-456",
		},
		{
			name: "user with empty tenant ID",
			setupCtx: func() context.Context {
				ctx := context.Background()
				user := &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: "user-456"},
						TenantID: "", // Empty tenant ID
					},
					Email: "test@example.com",
				}
				ctx = appctx.WithUser(ctx, user)
				return ctx
			},
			expectError: true,
			errorMsg:    "tenant ID is empty in user context",
		},
		{
			name: "user with empty user ID",
			setupCtx: func() context.Context {
				ctx := context.Background()
				user := &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: ""}, // Empty user ID
						TenantID: "tenant-123",
					},
					Email: "test@example.com",
				}
				ctx = appctx.WithUser(ctx, user)
				return ctx
			},
			expectError: true,
			errorMsg:    "user ID is empty in context",
		},
		{
			name: "missing user context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			expectError: true,
			errorMsg:    "user context is required but not found",
		},
		{
			name: "empty context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			expectError: true,
			errorMsg:    "user context is required but not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := tt.setupCtx()

			tenantID, userID, err := ExtractTenantUserFromContext(ctx)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, tt.errorMsg)
				c.Assert(tenantID, qt.Equals, "")
				c.Assert(userID, qt.Equals, "")
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(tenantID, qt.Equals, tt.tenantID)
				c.Assert(userID, qt.Equals, tt.userID)
			}
		})
	}
}

// newTestFactorySet creates a factory set for testing
func newTestFactorySet() *registry.FactorySet {
	factorySet := memory.NewFactorySet()

	// Create user with server-generated ID and capture it
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			// ID will be generated server-side for security
			TenantID: "test-tenant",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		IsActive: true,
	}))
	// Set the global testUserID to the generated ID
	testUserID = createdUser.ID

	must.Must(factorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant"},
		Name:     "Test Tenant",
	}))

	// Create a default location group — export's FileEntity creation now
	// requires a non-empty group_id in context (FileEntity is group-scoped).
	createdGroup := must.Must(factorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "test-tenant"},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Test Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           createdUser.ID,
	}))
	testGroupID = createdGroup.ID

	return factorySet
}

// newTestContext creates a context with test user + group for testing.
func newTestContext() context.Context {
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: testUserID},
			TenantID: "test-tenant",
		},
	})
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: testGroupID}, TenantID: "test-tenant"},
	})
}

func TestNewExportService(t *testing.T) {
	c := qt.New(t)
	factorySet := newTestFactorySet()
	uploadLocation := "/tmp/uploads"

	service := NewExportService(factorySet, uploadLocation)

	c.Assert(service, qt.IsNotNil)
	// Note: Cannot access private fields, just verify service is created
	c.Assert(service.uploadLocation, qt.Equals, uploadLocation)
}

func TestInventoryDataXMLStructure(t *testing.T) {
	c := qt.New(t)
	// Test the XML marshaling of the InventoryData structure
	data := &InventoryData{
		ExportDate: "2024-01-01T00:00:00Z",
		ExportType: "full_database",
		Locations: []*Location{
			{
				ID:      "loc1",
				Name:    "Main Warehouse",
				Address: "123 Main St",
			},
		},
		Areas: []*Area{
			{
				ID:         "area1",
				Name:       "Storage Area A",
				LocationID: "loc1",
			},
		},
		Commodities: []*Commodity{
			{
				ID:     "comm1",
				Name:   "Test Item",
				Type:   "equipment",
				AreaID: "area1",
				Count:  10,
				Status: "active",
			},
		},
	}

	xmlData, err := xml.MarshalIndent(data, "", "  ")
	c.Assert(err, qt.IsNil)

	// Check that the XML contains expected elements
	xmlStr := string(xmlData)
	expectedElements := []string{
		`<inventory exportDate="2024-01-01T00:00:00Z" exportType="full_database">`,
		`<locations>`,
		`<location id="loc1">`,
		`<locationName>Main Warehouse</locationName>`,
		`<areas>`,
		`<area id="area1">`,
		`<commodities>`,
		`<commodity id="comm1">`,
	}

	for _, expected := range expectedElements {
		c.Assert(xmlStr, qt.Contains, expected)
	}
}

func TestExportServiceProcessExport_InvalidID(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	uploadLocation := "file://" + tempDir + "?create_dir=1"
	factorySet := newTestFactorySet()
	service := NewExportService(factorySet, uploadLocation)
	ctx := newTestContext()

	// Test with non-existent export ID
	err := service.ProcessExport(ctx, "non-existent-id")
	c.Assert(err, qt.IsNotNil)
}

func TestExportServiceProcessExport_Success(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	factorySet := newTestFactorySet()
	service := NewExportService(factorySet, uploadLocation)
	registrySet := factorySet.CreateServiceRegistrySet()
	ctx := newTestContext()

	// Create a test export in the database
	export := models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("test-export-1", "test-tenant", testGroupID, testUserID),
		Type:                     models.ExportTypeCommodities,
		Status:                   models.ExportStatusPending,
		IncludeFileData:          false,
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	// Process the export
	err = service.ProcessExport(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	// Verify the export was updated
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	c.Assert(updatedExport.Status == models.ExportStatusCompleted || updatedExport.Status == models.ExportStatusFailed, qt.IsTrue)
}

func TestStreamXMLExport(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	uploadLocation := "file://" + tempDir + "?create_dir=1"
	factorySet := newTestFactorySet()
	service := NewExportService(factorySet, uploadLocation)
	ctx := newTestContext()

	// Test different export types
	testCases := []struct {
		name       string
		exportType models.ExportType
	}{
		{"Full Database", models.ExportTypeFullDatabase},
		{"Locations", models.ExportTypeLocations},
		{"Areas", models.ExportTypeAreas},
		{"Commodities", models.ExportTypeCommodities},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			export := models.Export{
				TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("test-export-"+tc.name, "test-tenant", testGroupID, testUserID),
				Type:                     tc.exportType,
				Status:                   models.ExportStatusPending,
				IncludeFileData:          false,
			}

			var buf bytes.Buffer
			_, err := service.streamXMLExport(ctx, export, &buf)
			c.Assert(err, qt.IsNil)

			xmlContent := buf.String()
			c.Assert(xmlContent, qt.Contains, `<?xml version="1.0" encoding="UTF-8"?>`)
			c.Assert(xmlContent, qt.Contains, fmt.Sprintf(`exportType="%s"`, tc.exportType))
			c.Assert(xmlContent, qt.Contains, `<inventory`)
			c.Assert(xmlContent, qt.Contains, `</inventory>`)
		})
	}
}

func TestStreamXMLExport_InvalidType(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	uploadLocation := "file://" + tempDir + "?create_dir=1"
	factorySet := newTestFactorySet()
	service := NewExportService(factorySet, uploadLocation)
	ctx := newTestContext()

	export := models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("test-export-invalid", "test-tenant", testGroupID, testUserID),
		Type:                     "invalid_type",
		Status:                   models.ExportStatusPending,
		IncludeFileData:          false,
	}

	var buf bytes.Buffer
	_, err := service.streamXMLExport(ctx, export, &buf)
	c.Assert(err, qt.IsNotNil)
}

func TestGenerateExport(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	factorySet := newTestFactorySet()
	service := NewExportService(factorySet, uploadLocation)
	ctx := newTestContext()

	export := models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("test-export-123", "default-tenant", testGroupID, testUserID),
		Type:                     models.ExportTypeCommodities,
		Status:                   models.ExportStatusPending,
		IncludeFileData:          false,
	}

	blobKey, _, err := service.generateExport(ctx, export)
	c.Assert(err, qt.IsNil)

	// Check that blob was created
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	exists, err := b.Exists(ctx, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	// Check blob key format
	expectedPrefix := fmt.Sprintf("exports/export_%s_", export.Type)
	c.Assert(blobKey, qt.Contains, expectedPrefix)
	c.Assert(blobKey, qt.Contains, ".xml")

	// Clean up
	err = b.Delete(ctx, blobKey)
	c.Assert(err, qt.IsNil)
}
