package export_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/export"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestExportService_ImportXMLExport(t *testing.T) {
	c := quicktest.New(t)

	// Create test registry
	_, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, quicktest.IsNil)

	// Test cases for happy path
	happyPathTests := []struct {
		name                   string
		xmlContent             string
		description            string
		expectedType           models.ExportType
		expectedLocationCount  int
		expectedAreaCount      int
		expectedCommodityCount int
	}{
		{
			name: "full database export",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<inventory exportType="full_database">
	<locations>
		<location id="loc1">
			<name>Location 1</name>
		</location>
		<location id="loc2">
			<name>Location 2</name>
		</location>
	</locations>
	<areas>
		<area id="area1">
			<name>Area 1</name>
			<location_id>loc1</location_id>
		</area>
	</areas>
	<commodities>
		<commodity id="comm1">
			<name>Commodity 1</name>
			<area_id>area1</area_id>
		</commodity>
	</commodities>
</inventory>`,
			description:            "Test full database import",
			expectedType:           models.ExportTypeFullDatabase,
			expectedLocationCount:  2,
			expectedAreaCount:      1,
			expectedCommodityCount: 1,
		},
		{
			name: "locations only export",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<inventory exportType="locations">
	<locations>
		<location id="loc1">
			<name>Location 1</name>
		</location>
	</locations>
</inventory>`,
			description:            "Test locations import",
			expectedType:           models.ExportTypeLocations,
			expectedLocationCount:  1,
			expectedAreaCount:      0,
			expectedCommodityCount: 0,
		},
		{
			name: "inferred type from content",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<inventory>
	<commodities>
		<commodity id="comm1">
			<name>Commodity 1</name>
		</commodity>
		<commodity id="comm2">
			<name>Commodity 2</name>
		</commodity>
	</commodities>
</inventory>`,
			description:            "Test inferred type",
			expectedType:           models.ExportTypeCommodities,
			expectedLocationCount:  0,
			expectedAreaCount:      0,
			expectedCommodityCount: 2,
		},
	}

	for _, tt := range happyPathTests {
		t.Run(tt.name, func(t *testing.T) {
			c := quicktest.New(t)

			// Create a mock XML file in memory
			// Note: In a real test, you would upload the file to the blob storage first
			// For this test, we'll need to mock the blob storage or create a test file

			// This test would need actual blob storage setup to work properly
			// For now, we'll test the parsing logic separately
			c.Skip("Requires blob storage setup for full integration test")
		})
	}
}

func TestExportService_parseXMLMetadata(t *testing.T) {
	c := quicktest.New(t)

	// Create test registry
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, quicktest.IsNil)
	service := export.NewExportService(registrySet, "mem://test-bucket")

	ctx := context.Background()

	// Test cases for happy path
	happyPathTests := []struct {
		name                   string
		xmlContent             string
		expectedType           models.ExportType
		expectedLocationCount  int
		expectedAreaCount      int
		expectedCommodityCount int
	}{
		{
			name: "full database export with explicit type",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<inventory exportType="full_database">
	<locations>
		<location id="loc1"><name>Location 1</name></location>
		<location id="loc2"><name>Location 2</name></location>
	</locations>
	<areas>
		<area id="area1"><name>Area 1</name></area>
	</areas>
	<commodities>
		<commodity id="comm1"><name>Commodity 1</name></commodity>
	</commodities>
</inventory>`,
			expectedType:           models.ExportTypeFullDatabase,
			expectedLocationCount:  2,
			expectedAreaCount:      1,
			expectedCommodityCount: 1,
		},
		{
			name: "locations only export",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<inventory exportType="locations">
	<locations>
		<location id="loc1"><name>Location 1</name></location>
	</locations>
</inventory>`,
			expectedType:           models.ExportTypeLocations,
			expectedLocationCount:  1,
			expectedAreaCount:      0,
			expectedCommodityCount: 0,
		},
		{
			name: "inferred commodities type",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<inventory>
	<commodities>
		<commodity id="comm1"><name>Commodity 1</name></commodity>
		<commodity id="comm2"><name>Commodity 2</name></commodity>
	</commodities>
</inventory>`,
			expectedType:           models.ExportTypeCommodities,
			expectedLocationCount:  0,
			expectedAreaCount:      0,
			expectedCommodityCount: 2,
		},
		{
			name: "inferred selected items type",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<inventory>
	<locations>
		<location id="loc1"><name>Location 1</name></location>
	</locations>
	<commodities>
		<commodity id="comm1"><name>Commodity 1</name></commodity>
	</commodities>
</inventory>`,
			expectedType:           models.ExportTypeSelectedItems,
			expectedLocationCount:  1,
			expectedAreaCount:      0,
			expectedCommodityCount: 1,
		},
	}

	for _, tt := range happyPathTests {
		t.Run(tt.name, func(t *testing.T) {
			c := quicktest.New(t)

			reader := strings.NewReader(tt.xmlContent)
			stats, exportType, err := service.ParseXMLMetadata(ctx, reader)

			c.Assert(err, quicktest.IsNil)
			c.Assert(exportType, quicktest.Equals, tt.expectedType)
			c.Assert(stats.LocationCount, quicktest.Equals, tt.expectedLocationCount)
			c.Assert(stats.AreaCount, quicktest.Equals, tt.expectedAreaCount)
			c.Assert(stats.CommodityCount, quicktest.Equals, tt.expectedCommodityCount)
		})
	}
}

// Test cases for unhappy path
func TestExportService_parseXMLMetadata_Errors(t *testing.T) {
	c := quicktest.New(t)

	// Create test registry
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, quicktest.IsNil)
	service := export.NewExportService(registrySet, "mem://test-bucket")

	ctx := context.Background()

	unhappyPathTests := []struct {
		name        string
		xmlContent  string
		expectError bool
	}{
		{
			name:        "invalid XML",
			xmlContent:  `<?xml version="1.0" encoding="UTF-8"?><invalid><unclosed>`,
			expectError: true,
		},
		{
			name:        "empty content",
			xmlContent:  "",
			expectError: false, // Empty content just returns empty stats
		},
	}

	for _, tt := range unhappyPathTests {
		t.Run(tt.name, func(t *testing.T) {
			c := quicktest.New(t)

			reader := strings.NewReader(tt.xmlContent)
			_, _, err := service.ParseXMLMetadata(ctx, reader)

			if tt.expectError {
				c.Assert(err, quicktest.IsNotNil)
			} else {
				c.Assert(err, quicktest.IsNil)
			}
		})
	}
}

func TestExportService_parseXMLMetadata_LargeFile(t *testing.T) {
	c := quicktest.New(t)

	// Create test registry
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, quicktest.IsNil)
	service := export.NewExportService(registrySet, "mem://test-bucket")

	ctx := context.Background()

	// Test with XML that includes large base64 data (simulated)
	largeBase64Data := strings.Repeat("SGVsbG8gV29ybGQ=", 10000) // Repeat base64 data many times
	xmlContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<inventory exportType="commodities">
	<commodities>
		<commodity id="comm1">
			<name>Commodity 1</name>
			<images>
				<file id="img1">
					<name>large-image.jpg</name>
					<data>%s</data>
				</file>
			</images>
			<invoices>
				<file id="inv1">
					<name>large-invoice.pdf</name>
					<data>%s</data>
				</file>
			</invoices>
		</commodity>
		<commodity id="comm2">
			<name>Commodity 2</name>
			<manuals>
				<file id="man1">
					<name>large-manual.pdf</name>
					<data>%s</data>
				</file>
			</manuals>
		</commodity>
	</commodities>
</inventory>`, largeBase64Data, largeBase64Data, largeBase64Data)

	reader := strings.NewReader(xmlContent)
	stats, exportType, err := service.ParseXMLMetadata(ctx, reader)

	c.Assert(err, quicktest.IsNil)
	c.Assert(exportType, quicktest.Equals, models.ExportTypeCommodities)
	c.Assert(stats.CommodityCount, quicktest.Equals, 2)
	c.Assert(stats.ImageCount, quicktest.Equals, 1)
	c.Assert(stats.InvoiceCount, quicktest.Equals, 1)
	c.Assert(stats.ManualCount, quicktest.Equals, 1)
	c.Assert(stats.LocationCount, quicktest.Equals, 0)
	c.Assert(stats.AreaCount, quicktest.Equals, 0)

	// Verify that binary data size was detected
	c.Assert(stats.BinaryDataSize > 0, quicktest.Equals, true, quicktest.Commentf("Expected binary data size to be detected, got %d", stats.BinaryDataSize))
}

func TestExportService_parseXMLMetadata_WithoutFileData(t *testing.T) {
	c := quicktest.New(t)

	// Create test registry
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, quicktest.IsNil)
	service := export.NewExportService(registrySet, "mem://test-bucket")

	ctx := context.Background()

	// Test with XML that includes files but no data elements
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<inventory exportType="commodities">
	<commodities>
		<commodity id="comm1">
			<name>Commodity 1</name>
			<images>
				<file id="img1">
					<name>image.jpg</name>
					<path>images/image.jpg</path>
				</file>
			</images>
			<invoices>
				<file id="inv1">
					<name>invoice.pdf</name>
					<path>invoices/invoice.pdf</path>
				</file>
			</invoices>
		</commodity>
	</commodities>
</inventory>`

	reader := strings.NewReader(xmlContent)
	stats, exportType, err := service.ParseXMLMetadata(ctx, reader)

	c.Assert(err, quicktest.IsNil)
	c.Assert(exportType, quicktest.Equals, models.ExportTypeCommodities)
	c.Assert(stats.CommodityCount, quicktest.Equals, 1)
	c.Assert(stats.ImageCount, quicktest.Equals, 1)
	c.Assert(stats.InvoiceCount, quicktest.Equals, 1)
	c.Assert(stats.ManualCount, quicktest.Equals, 0)

	// Verify that no binary data size was detected (files without data elements)
	c.Assert(stats.BinaryDataSize, quicktest.Equals, int64(0), quicktest.Commentf("Expected no binary data size for files without data, got %d", stats.BinaryDataSize))
}
