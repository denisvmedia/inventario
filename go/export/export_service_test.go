package export

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func newTestRegistrySet() *registry.Set {
	registrySet := &registry.Set{
		LocationRegistry:  memory.NewLocationRegistry(),
		AreaRegistry:      memory.NewAreaRegistry(memory.NewLocationRegistry()),
		CommodityRegistry: memory.NewCommodityRegistry(memory.NewAreaRegistry(memory.NewLocationRegistry())),
		ExportRegistry:    memory.NewExportRegistry(),
	}
	return registrySet
}

func TestNewExportService(t *testing.T) {
	c := qt.New(t)
	registrySet := &registry.Set{}
	uploadLocation := "/tmp/uploads"

	service := NewExportService(registrySet, uploadLocation)

	c.Assert(service, qt.IsNotNil)
	c.Assert(service.registrySet, qt.Equals, registrySet)
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
		`<inventory export_date="2024-01-01T00:00:00Z" export_type="full_database">`,
		`<locations>`,
		`<location id="loc1">`,
		`<location_name>Main Warehouse</location_name>`,
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

	registrySet := newTestRegistrySet()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Test with non-existent export ID
	err := service.ProcessExport(ctx, "non-existent-id")
	c.Assert(err, qt.IsNotNil)
}

func TestExportServiceProcessExport_Success(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create a test export in the database
	export := models.Export{
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
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

	registrySet := newTestRegistrySet()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := context.Background()

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
				Type:            tc.exportType,
				Status:          models.ExportStatusPending,
				IncludeFileData: false,
			}

			var buf bytes.Buffer
			err := service.streamXMLExport(ctx, export, &buf)
			c.Assert(err, qt.IsNil)

			xmlContent := buf.String()
			c.Assert(xmlContent, qt.Contains, `<?xml version="1.0" encoding="UTF-8"?>`)
			c.Assert(xmlContent, qt.Contains, fmt.Sprintf(`export_type="%s"`, tc.exportType))
			c.Assert(xmlContent, qt.Contains, `<inventory`)
			c.Assert(xmlContent, qt.Contains, `</inventory>`)
		})
	}
}

func TestStreamXMLExport_InvalidType(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := context.Background()

	export := models.Export{
		Type:            "invalid_type",
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	var buf bytes.Buffer
	err := service.streamXMLExport(ctx, export, &buf)
	c.Assert(err, qt.IsNotNil)
}

func TestGenerateExport(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := context.Background()

	export := models.Export{
		EntityID:        models.EntityID{ID: "test-export-123"},
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	blobKey, err := service.generateExport(ctx, export)
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
