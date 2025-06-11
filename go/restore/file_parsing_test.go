package restore_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/restore"

	// Import blob drivers
	_ "github.com/denisvmedia/inventario/internal/fileblob"
)

func TestRestoreService_FileElementParsing(t *testing.T) {
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

	// Create XML with <file> elements (the correct structure)
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
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
          <data>VGhpcyBpcyBhIHRlc3QgZmlsZSBjb250ZW50IGZvciB0ZXN0aW5nIGZpbGUgZGF0YSByZXN0b3JhdGlvbi4=</data>
        </file>
      </images>
    </commodity>
  </commodities>
</inventory>`

	// Create restore service with file:// blob storage for testing
	restoreService := restore.NewRestoreService(registrySet, "file://./test_uploads?create_dir=true")

	// Test restore with file data processing enabled
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		BackupExisting:  false,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader := strings.NewReader(xmlData)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)

	// Verify the basic data was restored correctly
	c.Assert(stats.LocationCount, qt.Equals, 1)
	c.Assert(stats.AreaCount, qt.Equals, 1)
	c.Assert(stats.CommodityCount, qt.Equals, 1)

	// Verify that file data was processed and file records were created successfully
	c.Assert(stats.BinaryDataSize > 0, qt.IsTrue, qt.Commentf("File data should be processed"))
	c.Assert(stats.ImageCount, qt.Equals, 1, qt.Commentf("Image should be created"))
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("No errors should occur"))

	// The file processing should now work correctly:
	// 1. The <file> element is correctly recognized
	// 2. The base64 data is successfully decoded
	// 3. The blob storage is working
	// 4. The file record is created after the commodity exists
	
	// Check that the commodity was created successfully
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)
	c.Assert(commodities[0].Name, qt.Equals, "Test Commodity")

	// Verify that the image is linked to the commodity
	imageIDs, err := registrySet.CommodityRegistry.GetImages(ctx, commodities[0].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(imageIDs, qt.HasLen, 1, qt.Commentf("Exactly one image should be linked to commodity"))

	// Verify the image record exists and has correct data
	image, err := registrySet.ImageRegistry.Get(ctx, imageIDs[0])
	c.Assert(err, qt.IsNil)
	c.Assert(image.CommodityID, qt.Equals, commodities[0].ID)
	c.Assert(image.Ext, qt.Equals, ".jpg")
	c.Assert(image.MIMEType, qt.Equals, "image/jpeg")

	// After the fix, both Path and OriginalPath should point to the generated filename
	c.Assert(image.Path, qt.Equals, image.OriginalPath, qt.Commentf("Path and OriginalPath should match for blob retrieval"))

	// The filename should follow the filekit.UploadFileName() format
	c.Assert(strings.HasPrefix(image.Path, "test-image-original-"), qt.IsTrue, qt.Commentf("Path should start with original filename"))
	c.Assert(strings.HasSuffix(image.Path, ".jpg"), qt.IsTrue, qt.Commentf("Path should end with extension"))

	// The image should have a generated ID (not the XML ID)
	c.Assert(image.ID, qt.Not(qt.Equals), "test-image-1")
	c.Assert(image.ID, qt.Not(qt.Equals), "")
}

func TestRestoreService_FileElementParsing_WithoutFileData(t *testing.T) {
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

	// Create XML with <file> elements but disable file processing
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
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
          <data>VGhpcyBpcyBhIHRlc3QgZmlsZSBjb250ZW50IGZvciB0ZXN0aW5nIGZpbGUgZGF0YSByZXN0b3JhdGlvbi4=</data>
        </file>
      </images>
    </commodity>
  </commodities>
</inventory>`

	// Create restore service
	restoreService := restore.NewRestoreService(registrySet, "file://./test_uploads?create_dir=true")

	// Test restore with file data processing DISABLED
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		BackupExisting:  false,
		DryRun:          false,
		IncludeFileData: false, // Disable file processing
	}

	reader := strings.NewReader(xmlData)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)

	// Verify the basic data was restored correctly
	c.Assert(stats.LocationCount, qt.Equals, 1)
	c.Assert(stats.AreaCount, qt.Equals, 1)
	c.Assert(stats.CommodityCount, qt.Equals, 1)

	// When file processing is disabled, no file data should be processed
	c.Assert(stats.BinaryDataSize, qt.Equals, int64(0))
	c.Assert(stats.ImageCount, qt.Equals, 0)

	// But the commodity should still be created successfully
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)
	c.Assert(commodities[0].Name, qt.Equals, "Test Commodity")
}

func TestRestoreService_PriceValidationFix(t *testing.T) {
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

	// Create XML with commodity that has original price in main currency but also has converted price
	// This simulates the issue where exported data contains both values
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
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
      <convertedOriginalPrice>100.00</convertedOriginalPrice>
      <currentPrice>120.00</currentPrice>
      <status>in_use</status>
      <purchaseDate>2024-01-01</purchaseDate>
      <registeredDate>2024-01-01</registeredDate>
      <lastModifiedDate>2024-01-01</lastModifiedDate>
      <draft>false</draft>
    </commodity>
  </commodities>
</inventory>`

	// Create restore service
	restoreService := restore.NewRestoreService(registrySet, "file://./test_uploads?create_dir=true")

	// Test restore with full replace strategy
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		BackupExisting:  false,
		DryRun:          false,
		IncludeFileData: false,
	}

	reader := strings.NewReader(xmlData)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)

	// Verify no validation errors occurred
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("No validation errors should occur"))
	c.Assert(stats.CommodityCount, qt.Equals, 1, qt.Commentf("Commodity should be created"))

	// Verify the commodity was created with corrected price
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)

	commodity := commodities[0]
	c.Assert(commodity.Name, qt.Equals, "Test Commodity")
	c.Assert(string(commodity.OriginalPriceCurrency), qt.Equals, "USD")
	c.Assert(commodity.OriginalPrice.String(), qt.Equals, "100")
	c.Assert(commodity.CurrentPrice.String(), qt.Equals, "120")

	// The key test: converted original price should be auto-corrected to zero
	// when original currency matches main currency
	c.Assert(commodity.ConvertedOriginalPrice.IsZero(), qt.IsTrue,
		qt.Commentf("ConvertedOriginalPrice should be auto-corrected to zero when original currency matches main currency"))
}

func TestRestoreService_NoDuplicationInFullReplace(t *testing.T) {
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

	// Create XML with multiple entities to test for duplication
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
  <locations>
    <location id="test-location-1">
      <locationName>Test Location 1</locationName>
      <address>123 Test Street</address>
    </location>
    <location id="test-location-2">
      <locationName>Test Location 2</locationName>
      <address>456 Test Avenue</address>
    </location>
  </locations>
  <areas>
    <area id="test-area-1">
      <areaName>Test Area 1</areaName>
      <locationId>test-location-1</locationId>
    </area>
    <area id="test-area-2">
      <areaName>Test Area 2</areaName>
      <locationId>test-location-2</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="test-commodity-1">
      <commodityName>Test Commodity 1</commodityName>
      <shortName>TestComm1</shortName>
      <type>equipment</type>
      <areaId>test-area-1</areaId>
      <count>1</count>
      <originalPrice>100.00</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <currentPrice>120.00</currentPrice>
      <status>in_use</status>
      <purchaseDate>2024-01-01</purchaseDate>
      <registeredDate>2024-01-01</registeredDate>
      <lastModifiedDate>2024-01-01</lastModifiedDate>
      <draft>false</draft>
    </commodity>
    <commodity id="test-commodity-2">
      <commodityName>Test Commodity 2</commodityName>
      <shortName>TestComm2</shortName>
      <type>equipment</type>
      <areaId>test-area-2</areaId>
      <count>1</count>
      <originalPrice>200.00</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <currentPrice>220.00</currentPrice>
      <status>in_use</status>
      <purchaseDate>2024-01-01</purchaseDate>
      <registeredDate>2024-01-01</registeredDate>
      <lastModifiedDate>2024-01-01</lastModifiedDate>
      <draft>false</draft>
    </commodity>
  </commodities>
</inventory>`

	// Create restore service
	restoreService := restore.NewRestoreService(registrySet, "file://./test_uploads?create_dir=true")

	// Test restore with full replace strategy
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		BackupExisting:  false,
		DryRun:          false,
		IncludeFileData: false,
	}

	reader := strings.NewReader(xmlData)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)

	// Verify no duplication occurred - exact counts should match XML
	c.Assert(stats.LocationCount, qt.Equals, 2, qt.Commentf("Should create exactly 2 locations"))
	c.Assert(stats.AreaCount, qt.Equals, 2, qt.Commentf("Should create exactly 2 areas"))
	c.Assert(stats.CommodityCount, qt.Equals, 2, qt.Commentf("Should create exactly 2 commodities"))
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("No errors should occur"))

	// Verify actual database counts match stats
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, 2, qt.Commentf("Database should contain exactly 2 locations"))

	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 2, qt.Commentf("Database should contain exactly 2 areas"))

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 2, qt.Commentf("Database should contain exactly 2 commodities"))
}

func TestRestoreService_MultipleFileTypes(t *testing.T) {
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

	// Create XML with multiple file types
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
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

	// Create restore service with file:// blob storage for testing
	restoreService := restore.NewRestoreService(registrySet, "file://./test_uploads?create_dir=true")

	// Test restore with file data processing enabled
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		BackupExisting:  false,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader := strings.NewReader(xmlData)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)

	// Verify all data was restored correctly
	c.Assert(stats.LocationCount, qt.Equals, 1)
	c.Assert(stats.AreaCount, qt.Equals, 1)
	c.Assert(stats.CommodityCount, qt.Equals, 1)
	c.Assert(stats.ImageCount, qt.Equals, 2)
	c.Assert(stats.InvoiceCount, qt.Equals, 1)
	c.Assert(stats.ManualCount, qt.Equals, 1)
	c.Assert(stats.ErrorCount, qt.Equals, 0)
	c.Assert(stats.BinaryDataSize > 0, qt.IsTrue)

	// Verify the commodity exists
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)

	// Verify all files are linked to the commodity
	imageIDs, err := registrySet.CommodityRegistry.GetImages(ctx, commodities[0].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(imageIDs, qt.HasLen, 2)

	invoiceIDs, err := registrySet.CommodityRegistry.GetInvoices(ctx, commodities[0].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(invoiceIDs, qt.HasLen, 1)

	manualIDs, err := registrySet.CommodityRegistry.GetManuals(ctx, commodities[0].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(manualIDs, qt.HasLen, 1)

	// Verify file records have correct metadata
	for _, imageID := range imageIDs {
		image, err := registrySet.ImageRegistry.Get(ctx, imageID)
		c.Assert(err, qt.IsNil)
		c.Assert(image.CommodityID, qt.Equals, commodities[0].ID)
		c.Assert(image.MIMEType, qt.Contains, "image/")
	}

	invoice, err := registrySet.InvoiceRegistry.Get(ctx, invoiceIDs[0])
	c.Assert(err, qt.IsNil)
	c.Assert(invoice.CommodityID, qt.Equals, commodities[0].ID)
	c.Assert(invoice.MIMEType, qt.Equals, "application/pdf")

	manual, err := registrySet.ManualRegistry.Get(ctx, manualIDs[0])
	c.Assert(err, qt.IsNil)
	c.Assert(manual.CommodityID, qt.Equals, commodities[0].ID)
	c.Assert(manual.MIMEType, qt.Equals, "application/pdf")
}
