package export

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

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
	exportDir := "/tmp/exports"
	uploadLocation := "/tmp/uploads"

	service := NewExportService(registrySet, exportDir, uploadLocation)

	c.Assert(service, qt.Not(qt.IsNil))
	c.Assert(service.registrySet, qt.Equals, registrySet)
	c.Assert(service.exportDir, qt.Equals, exportDir)
	c.Assert(service.uploadLocation, qt.Equals, uploadLocation)
}

func TestInventoryDataXMLStructure(t *testing.T) {
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
	if err != nil {
		t.Fatalf("Failed to marshal XML: %v", err)
	}

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
		if !strings.Contains(xmlStr, expected) {
			t.Errorf("Expected XML to contain %q, but it didn't. XML:\n%s", expected, xmlStr)
		}
	}
}

func TestExportServiceProcessExport_InvalidID(t *testing.T) {
	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	registrySet := newTestRegistrySet()
	service := NewExportService(registrySet, tempDir, "/tmp/uploads")
	ctx := context.Background()

	// Test with non-existent export ID
	err = service.ProcessExport(ctx, "non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent export ID, got nil")
	}
}

func TestExportServiceProcessExport_Success(t *testing.T) {
	// Create a temporary directory for exports
	tempDir := t.TempDir()

	registrySet := newTestRegistrySet()
	service := NewExportService(registrySet, tempDir, "/tmp/uploads")
	ctx := context.Background()

	// Create a test export in the database
	export := models.Export{
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	if err != nil {
		t.Fatalf("Failed to create export: %v", err)
	}

	// Process the export
	err = service.ProcessExport(ctx, createdExport.ID)
	if err != nil {
		t.Errorf("Unexpected error processing export: %v", err)
	}

	// Verify the export was updated
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	if err != nil {
		t.Fatalf("Failed to get updated export: %v", err)
	}

	if updatedExport.Status != models.ExportStatusCompleted && updatedExport.Status != models.ExportStatusFailed {
		t.Errorf("Expected export status to be completed or failed, got %s", updatedExport.Status)
	}
}

func TestGenerateXMLData(t *testing.T) {
	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	registrySet := newTestRegistrySet()
	service := NewExportService(registrySet, tempDir, "/tmp/uploads")
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
			export := models.Export{
				Type:            tc.exportType,
				Status:          models.ExportStatusPending,
				IncludeFileData: false,
			}

			data, err := service.generateXMLData(ctx, export)
			if err != nil {
				t.Errorf("Unexpected error generating XML data: %v", err)
				return
			}

			if data == nil {
				t.Error("Expected XML data, got nil")
				return
			}

			if data.ExportType != string(tc.exportType) {
				t.Errorf("Expected export type %s, got %s", tc.exportType, data.ExportType)
			}

			if data.ExportDate == "" {
				t.Error("Expected export date to be set")
			}
		})
	}
}

func TestGenerateXMLData_InvalidType(t *testing.T) {
	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	registrySet := newTestRegistrySet()
	service := NewExportService(registrySet, tempDir, "/tmp/uploads")
	ctx := context.Background()

	export := models.Export{
		Type:            "invalid_type",
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	_, err = service.generateXMLData(ctx, export)
	if err == nil {
		t.Error("Expected error for invalid export type, got nil")
	}
}

func TestGenerateExport(t *testing.T) {
	// Create a temporary directory for exports
	tempDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	registrySet := newTestRegistrySet()
	service := NewExportService(registrySet, tempDir, "/tmp/uploads")
	ctx := context.Background()

	export := models.Export{
		EntityID:        models.EntityID{ID: "test-export-123"},
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	filePath, err := service.generateExport(ctx, export)
	if err != nil {
		t.Fatalf("Unexpected error generating export: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Export file was not created at %s", filePath)
	}

	// Check file name format
	expectedPrefix := fmt.Sprintf("export_%s_", export.Type)
	fileName := filepath.Base(filePath)
	if !strings.Contains(fileName, expectedPrefix) {
		t.Errorf("Expected file name to contain %s, got %s", expectedPrefix, fileName)
	}

	if !strings.Contains(fileName, ".xml") {
		t.Errorf("Expected file name to have .xml extension, got %s", fileName)
	}

	// Clean up
	os.Remove(filePath)
}

