package export_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/backup/export"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestExportService_parseXMLMetadata(t *testing.T) {
	c := qt.New(t)

	// Create test registry
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)
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
	}

	for _, tt := range happyPathTests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			reader := strings.NewReader(tt.xmlContent)
			stats, exportType, err := service.ParseXMLMetadata(ctx, reader)

			c.Assert(err, qt.IsNil)
			c.Assert(exportType, qt.Equals, tt.expectedType)
			c.Assert(stats.LocationCount, qt.Equals, tt.expectedLocationCount)
			c.Assert(stats.AreaCount, qt.Equals, tt.expectedAreaCount)
			c.Assert(stats.CommodityCount, qt.Equals, tt.expectedCommodityCount)
		})
	}
}

// Test cases for unhappy path
func TestExportService_parseXMLMetadata_Errors(t *testing.T) {
	c := qt.New(t)

	// Create test registry
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)
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
			c := qt.New(t)

			reader := strings.NewReader(tt.xmlContent)
			_, _, err := service.ParseXMLMetadata(ctx, reader)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestExportService_parseXMLMetadata_LargeFile(t *testing.T) {
	c := qt.New(t)

	// Create test registry
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)
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

	c.Assert(err, qt.IsNil)
	c.Assert(exportType, qt.Equals, models.ExportTypeCommodities)
	c.Assert(stats.CommodityCount, qt.Equals, 2)
	c.Assert(stats.ImageCount, qt.Equals, 1)
	c.Assert(stats.InvoiceCount, qt.Equals, 1)
	c.Assert(stats.ManualCount, qt.Equals, 1)
	c.Assert(stats.LocationCount, qt.Equals, 0)
	c.Assert(stats.AreaCount, qt.Equals, 0)

	// Verify that binary data size was detected
	c.Assert(stats.BinaryDataSize > 0, qt.Equals, true, qt.Commentf("Expected binary data size to be detected, got %d", stats.BinaryDataSize))
}

func TestExportService_parseXMLMetadata_WithoutFileData(t *testing.T) {
	c := qt.New(t)

	// Create test registry
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)
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

	c.Assert(err, qt.IsNil)
	c.Assert(exportType, qt.Equals, models.ExportTypeCommodities)
	c.Assert(stats.CommodityCount, qt.Equals, 1)
	c.Assert(stats.ImageCount, qt.Equals, 1)
	c.Assert(stats.InvoiceCount, qt.Equals, 1)
	c.Assert(stats.ManualCount, qt.Equals, 0)

	// Verify that no binary data size was detected (files without data elements)
	c.Assert(stats.BinaryDataSize, qt.Equals, int64(0), qt.Commentf("Expected no binary data size for files without data, got %d", stats.BinaryDataSize))
}
