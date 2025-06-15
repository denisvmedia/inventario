package restore_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/backup/restore"

	// Import blob drivers
	_ "github.com/denisvmedia/inventario/internal/fileblob"
)

func TestRestoreService_MergeAddStrategy_NoDuplicateFiles(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registry set with proper dependencies
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)

	// Set up main currency in settings (required for commodity validation)
	mainCurrency := "USD"
	settings := models.SettingsObject{
		MainCurrency: &mainCurrency,
	}
	err = registrySet.SettingsRegistry.Save(ctx, settings)
	c.Assert(err, qt.IsNil)

	// Create restore service
	restoreService := restore.NewRestoreService(registrySet, "file://./test_uploads?create_dir=true")

	// First, create some initial data with files
	initialXML := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
  <locations>
    <location id="test-location-1">
      <locationName>Test Location</locationName>
      <address>123 Test Street</address>
    </location>
  </locations>
  <areas>
    <area id="test-area-1">
      <areaName>Test Area</areaName>
      <locationId>test-location-1</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="test-commodity-1">
      <commodityName>Test Commodity</commodityName>
      <shortName>TestComm</shortName>
      <type>equipment</type>
      <areaId>test-area-1</areaId>
      <count>1</count>
      <originalPrice>100.00</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <currentPrice>100.00</currentPrice>
      <status>in_use</status>
      <purchaseDate>2024-01-01</purchaseDate>
      <registeredDate>2024-01-01</registeredDate>
      <lastModifiedDate>2024-01-01</lastModifiedDate>
      <draft>false</draft>
      <images>
        <file id="test-image-1">
          <path>test-image</path>
          <originalPath>test-image-original.jpg</originalPath>
          <extension>.jpg</extension>
          <mimeType>image/jpeg</mimeType>
          <data>VGhpcyBpcyBhIHRlc3QgaW1hZ2UgZmlsZSBjb250ZW50Lg==</data>
        </file>
      </images>
      <invoices>
        <file id="test-invoice-1">
          <path>test-invoice</path>
          <originalPath>test-invoice-original.pdf</originalPath>
          <extension>.pdf</extension>
          <mimeType>application/pdf</mimeType>
          <data>VGhpcyBpcyBhIHRlc3QgaW52b2ljZSBmaWxlIGNvbnRlbnQu</data>
        </file>
      </invoices>
      <manuals>
        <file id="test-manual-1">
          <path>test-manual</path>
          <originalPath>test-manual-original.pdf</originalPath>
          <extension>.pdf</extension>
          <mimeType>application/pdf</mimeType>
          <data>VGhpcyBpcyBhIHRlc3QgbWFudWFsIGZpbGUgY29udGVudC4=</data>
        </file>
      </manuals>
    </commodity>
  </commodities>
</inventory>`

	// First restore with full replace to create initial data
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader := strings.NewReader(initialXML)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0)

	// Verify initial data was created
	c.Assert(stats.LocationCount, qt.Equals, 1)
	c.Assert(stats.AreaCount, qt.Equals, 1)
	c.Assert(stats.CommodityCount, qt.Equals, 1)
	c.Assert(stats.ImageCount, qt.Equals, 1)
	c.Assert(stats.InvoiceCount, qt.Equals, 1)
	c.Assert(stats.ManualCount, qt.Equals, 1)

	// Get initial counts from database
	initialImages, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	initialImageCount := len(initialImages)

	initialInvoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	initialInvoiceCount := len(initialInvoices)

	initialManuals, err := registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	initialManualCount := len(initialManuals)

	c.Assert(initialImageCount, qt.Equals, 1)
	c.Assert(initialInvoiceCount, qt.Equals, 1)
	c.Assert(initialManualCount, qt.Equals, 1)

	// Now try to restore the same data again using Merge & Add strategy
	// This should NOT create duplicates
	mergeAddOptions := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyMergeAdd,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader2 := strings.NewReader(initialXML)
	stats2, err := restoreService.RestoreFromXML(ctx, reader2, mergeAddOptions)
	c.Assert(err, qt.IsNil)
	c.Assert(stats2.ErrorCount, qt.Equals, 0)

	// With Merge & Add, existing items should be skipped, not duplicated
	c.Assert(stats2.CreatedCount, qt.Equals, 0, qt.Commentf("No new items should be created"))
	c.Assert(stats2.SkippedCount > 0, qt.IsTrue, qt.Commentf("Items should be skipped"))

	// Verify no new files were created
	c.Assert(stats2.ImageCount, qt.Equals, 0, qt.Commentf("No new images should be created"))
	c.Assert(stats2.InvoiceCount, qt.Equals, 0, qt.Commentf("No new invoices should be created"))
	c.Assert(stats2.ManualCount, qt.Equals, 0, qt.Commentf("No new manuals should be created"))

	// Verify database counts remain the same
	finalImages, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalImages), qt.Equals, initialImageCount, qt.Commentf("Image count should remain the same"))

	finalInvoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalInvoices), qt.Equals, initialInvoiceCount, qt.Commentf("Invoice count should remain the same"))

	finalManuals, err := registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalManuals), qt.Equals, initialManualCount, qt.Commentf("Manual count should remain the same"))
}

func TestRestoreService_MergeAddStrategy_AddNewFilesOnly(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registry set with proper dependencies
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)

	// Set up main currency in settings (required for commodity validation)
	mainCurrency := "USD"
	settings := models.SettingsObject{
		MainCurrency: &mainCurrency,
	}
	err = registrySet.SettingsRegistry.Save(ctx, settings)
	c.Assert(err, qt.IsNil)

	// Create restore service
	restoreService := restore.NewRestoreService(registrySet, "file://./test_uploads?create_dir=true")

	// First, create initial data with one file
	initialXML := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
  <locations>
    <location id="test-location-1">
      <locationName>Test Location</locationName>
      <address>123 Test Street</address>
    </location>
  </locations>
  <areas>
    <area id="test-area-1">
      <areaName>Test Area</areaName>
      <locationId>test-location-1</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="test-commodity-1">
      <commodityName>Test Commodity</commodityName>
      <shortName>TestComm</shortName>
      <type>equipment</type>
      <areaId>test-area-1</areaId>
      <count>1</count>
      <originalPrice>100.00</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <currentPrice>100.00</currentPrice>
      <status>in_use</status>
      <purchaseDate>2024-01-01</purchaseDate>
      <registeredDate>2024-01-01</registeredDate>
      <lastModifiedDate>2024-01-01</lastModifiedDate>
      <draft>false</draft>
      <images>
        <file id="test-image-1">
          <path>test-image</path>
          <originalPath>test-image-original.jpg</originalPath>
          <extension>.jpg</extension>
          <mimeType>image/jpeg</mimeType>
          <data>VGhpcyBpcyBhIHRlc3QgaW1hZ2UgZmlsZSBjb250ZW50Lg==</data>
        </file>
      </images>
    </commodity>
  </commodities>
</inventory>`

	// First restore with full replace to create initial data
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader := strings.NewReader(initialXML)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0)
	c.Assert(stats.ImageCount, qt.Equals, 1)

	// Get initial counts
	initialImages, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	initialImageCount := len(initialImages)
	c.Assert(initialImageCount, qt.Equals, 1)

	// Now restore data with additional files using Merge & Add strategy
	xmlWithNewFiles := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
  <locations>
    <location id="test-location-1">
      <locationName>Test Location</locationName>
      <address>123 Test Street</address>
    </location>
  </locations>
  <areas>
    <area id="test-area-1">
      <areaName>Test Area</areaName>
      <locationId>test-location-1</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="test-commodity-1">
      <commodityName>Test Commodity</commodityName>
      <shortName>TestComm</shortName>
      <type>equipment</type>
      <areaId>test-area-1</areaId>
      <count>1</count>
      <originalPrice>100.00</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <currentPrice>100.00</currentPrice>
      <status>in_use</status>
      <purchaseDate>2024-01-01</purchaseDate>
      <registeredDate>2024-01-01</registeredDate>
      <lastModifiedDate>2024-01-01</lastModifiedDate>
      <draft>false</draft>
      <images>
        <file id="test-image-1">
          <path>test-image</path>
          <originalPath>test-image-original.jpg</originalPath>
          <extension>.jpg</extension>
          <mimeType>image/jpeg</mimeType>
          <data>VGhpcyBpcyBhIHRlc3QgaW1hZ2UgZmlsZSBjb250ZW50Lg==</data>
        </file>
        <file id="test-image-2">
          <path>test-image-2</path>
          <originalPath>test-image-2-original.png</originalPath>
          <extension>.png</extension>
          <mimeType>image/png</mimeType>
          <data>VGhpcyBpcyBhbm90aGVyIHRlc3QgaW1hZ2UgZmlsZSBjb250ZW50Lg==</data>
        </file>
      </images>
      <invoices>
        <file id="test-invoice-1">
          <path>test-invoice</path>
          <originalPath>test-invoice-original.pdf</originalPath>
          <extension>.pdf</extension>
          <mimeType>application/pdf</mimeType>
          <data>VGhpcyBpcyBhIHRlc3QgaW52b2ljZSBmaWxlIGNvbnRlbnQu</data>
        </file>
      </invoices>
    </commodity>
  </commodities>
</inventory>`

	// Restore with Merge & Add strategy
	mergeAddOptions := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyMergeAdd,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader2 := strings.NewReader(xmlWithNewFiles)
	stats2, err := restoreService.RestoreFromXML(ctx, reader2, mergeAddOptions)
	c.Assert(err, qt.IsNil)
	c.Assert(stats2.ErrorCount, qt.Equals, 0)

	// Should create only the new files, not duplicate existing ones
	c.Assert(stats2.ImageCount, qt.Equals, 1, qt.Commentf("Should create 1 new image (test-image-2)"))
	c.Assert(stats2.InvoiceCount, qt.Equals, 1, qt.Commentf("Should create 1 new invoice"))

	// Verify final counts in database
	finalImages, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalImages), qt.Equals, 2, qt.Commentf("Should have 2 images total (1 existing + 1 new)"))

	finalInvoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalInvoices), qt.Equals, 1, qt.Commentf("Should have 1 invoice total"))

	// Verify that the existing image is still there and the new one was added
	imageIDs := make(map[string]bool)
	for _, img := range finalImages {
		imageIDs[img.ID] = true
	}
	c.Assert(len(imageIDs), qt.Equals, 2, qt.Commentf("Should have 2 unique image IDs"))
}
