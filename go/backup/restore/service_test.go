package restore_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/processor"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func TestRestoreService_RestoreFromXML(t *testing.T) {
	ctx := validationctx.WithMainCurrency(t.Context(), "USD")
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})

	t.Run("restore XML with full replace strategy", func(t *testing.T) {
		c := qt.New(t)

		// Create fresh registry set for this test
		testRegistrySet := memory.NewRegistrySet()
		c.Assert(testRegistrySet, qt.IsNotNil)
		entityService := services.NewEntityService(testRegistrySet, "/tmp/test-uploads")
		proc := processor.NewRestoreOperationProcessor("test-op", testRegistrySet, entityService, "/tmp/test-uploads")

		xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
	<locations>
		<location id="loc1">
			<locationName>Test Location</locationName>
			<address>123 Test St</address>
		</location>
	</locations>
	<areas>
		<area id="area1">
			<areaName>Test Area</areaName>
			<locationId>loc1</locationId>
		</area>
	</areas>
	<commodities>
		<commodity id="comm1">
			<commodityName>Test Commodity</commodityName>
			<shortName>TC</shortName>
			<areaId>area1</areaId>
			<count>5</count>
			<status>in_use</status>
			<type>electronics</type>
			<originalPrice>100.00</originalPrice>
			<originalPriceCurrency>USD</originalPriceCurrency>
			<convertedOriginalPrice>0</convertedOriginalPrice>
			<currentPrice>90.00</currentPrice>
			<purchaseDate>2023-01-01</purchaseDate>
		</commodity>
	</commodities>
</inventory>`

		options := types.RestoreOptions{
			Strategy:        types.RestoreStrategyFullReplace,
			IncludeFileData: false,
			DryRun:          false,
		}

		stats, err := proc.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
		c.Assert(err, qt.IsNil)
		c.Assert(stats, qt.IsNotNil)

		// Debug output
		c.Logf("Stats: LocationCount=%d, AreaCount=%d, CommodityCount=%d, CreatedCount=%d, ErrorCount=%d",
			stats.LocationCount, stats.AreaCount, stats.CommodityCount, stats.CreatedCount, stats.ErrorCount)
		if len(stats.Errors) > 0 {
			c.Logf("Errors: %v", stats.Errors)
		}

		// Debug: Check if location was actually created
		locations, locErr := testRegistrySet.LocationRegistry.List(ctx)
		c.Assert(locErr, qt.IsNil)
		c.Logf("Locations in database after restore: %d", len(locations))
		for _, loc := range locations {
			c.Logf("Location: ID=%s, Name=%s", loc.ID, loc.Name)
		}

		c.Assert(stats.LocationCount, qt.Equals, 1)
		c.Assert(stats.AreaCount, qt.Equals, 1)
		c.Assert(stats.CommodityCount, qt.Equals, 1)
		c.Assert(stats.ErrorCount, qt.Equals, 0)
		c.Assert(stats.CreatedCount, qt.Equals, 3) // 1 location + 1 area + 1 commodity

		// Verify data was created
		locations, err = testRegistrySet.LocationRegistry.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(locations), qt.Equals, 1)
		c.Assert(locations[0].Name, qt.Equals, "Test Location")

		areas, err := testRegistrySet.AreaRegistry.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(areas), qt.Equals, 1)
		c.Assert(areas[0].Name, qt.Equals, "Test Area")

		commodities, err := testRegistrySet.CommodityRegistry.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(commodities), qt.Equals, 1)
		c.Assert(commodities[0].Name, qt.Equals, "Test Commodity")
	})

	t.Run("restore XML with merge add strategy", func(t *testing.T) {
		c := qt.New(t)

		// Create fresh registry set for this test
		testRegistrySet := memory.NewRegistrySet()
		entityService := services.NewEntityService(testRegistrySet, "/tmp/test-uploads")
		proc := processor.NewRestoreOperationProcessor("test-op", testRegistrySet, entityService, "/tmp/test-uploads")

		// First, create some existing data
		existingLocation := models.Location{
			TenantAwareEntityID: models.WithTenantAwareEntityID("loc1", "default-tenant"),
			Name:                "Existing Location",
			Address:             "456 Existing St",
		}
		createdLocation, err := testRegistrySet.LocationRegistry.Create(ctx, existingLocation)
		c.Assert(err, qt.IsNil)
		c.Logf("Created existing location with ID: %s (original ID was: %s)", createdLocation.ID, existingLocation.ID)

		xmlData := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
	<locations>
		<location id="%s">
			<locationName>Test Location</locationName>
			<address>123 Test St</address>
		</location>
		<location id="loc2">
			<locationName>New Location</locationName>
			<address>789 New St</address>
		</location>
	</locations>
</inventory>`, createdLocation.ID)

		options := types.RestoreOptions{
			Strategy:        types.RestoreStrategyMergeAdd,
			IncludeFileData: false,
			DryRun:          false,
		}

		stats, err := proc.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
		c.Assert(err, qt.IsNil)
		c.Assert(stats, qt.IsNotNil)
		c.Assert(stats.LocationCount, qt.Equals, 1) // Only new location counted
		c.Assert(stats.ErrorCount, qt.Equals, 0)
		c.Assert(stats.CreatedCount, qt.Equals, 1) // Only new location created
		c.Assert(stats.SkippedCount, qt.Equals, 1) // Existing location skipped

		// Verify data - should have 2 locations total
		locations, err := testRegistrySet.LocationRegistry.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(locations), qt.Equals, 2)

		// Verify existing location was not modified
		existingLoc, err := testRegistrySet.LocationRegistry.Get(ctx, createdLocation.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(existingLoc.Name, qt.Equals, "Existing Location") // Should remain unchanged
	})

	t.Run("restore XML with merge update strategy", func(t *testing.T) {
		c := qt.New(t)

		// Create fresh registry set for this test
		testRegistrySet := memory.NewRegistrySet()
		entityService := services.NewEntityService(testRegistrySet, "/tmp/test-uploads")
		proc := processor.NewRestoreOperationProcessor("test-op", testRegistrySet, entityService, "/tmp/test-uploads")

		// First, create some existing data
		existingLocation := models.Location{
			TenantAwareEntityID: models.WithTenantAwareEntityID("loc1", "default-tenant"),
			Name:                "Existing Location",
			Address:             "456 Existing St",
		}
		createdLocation, err := testRegistrySet.LocationRegistry.Create(ctx, existingLocation)
		c.Assert(err, qt.IsNil)

		xmlData := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
	<locations>
		<location id="%s">
			<locationName>Updated Location</locationName>
			<address>123 Updated St</address>
		</location>
		<location id="loc2">
			<locationName>New Location</locationName>
			<address>789 New St</address>
		</location>
	</locations>
</inventory>`, createdLocation.ID)

		options := types.RestoreOptions{
			Strategy:        types.RestoreStrategyMergeUpdate,
			IncludeFileData: false,
			DryRun:          false,
		}

		stats, err := proc.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
		c.Assert(err, qt.IsNil)
		c.Assert(stats, qt.IsNotNil)
		c.Assert(stats.LocationCount, qt.Equals, 2) // Both locations processed
		c.Assert(stats.ErrorCount, qt.Equals, 0)
		c.Assert(stats.CreatedCount, qt.Equals, 1) // New location created
		c.Assert(stats.UpdatedCount, qt.Equals, 1) // Existing location updated

		// Verify data - should have 2 locations total
		locations, err := testRegistrySet.LocationRegistry.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(locations), qt.Equals, 2)

		// Verify existing location was updated
		updatedLoc, err := testRegistrySet.LocationRegistry.Get(ctx, createdLocation.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(updatedLoc.Name, qt.Equals, "Updated Location") // Should be updated
		c.Assert(updatedLoc.Address, qt.Equals, "123 Updated St")
	})

	t.Run("dry run mode", func(t *testing.T) {
		c := qt.New(t)

		// Create fresh registry set for this test
		testRegistrySet := memory.NewRegistrySet()
		entityService := services.NewEntityService(testRegistrySet, "/tmp/test-uploads")
		proc := processor.NewRestoreOperationProcessor("test-op", testRegistrySet, entityService, "/tmp/test-uploads")

		xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
	<locations>
		<location id="loc1">
			<locationName>Test Location</locationName>
			<address>123 Test St</address>
		</location>
	</locations>
</inventory>`

		options := types.RestoreOptions{
			Strategy:        types.RestoreStrategyFullReplace,
			IncludeFileData: false,
			DryRun:          true, // Dry run mode
		}

		stats, err := proc.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
		c.Assert(err, qt.IsNil)
		c.Assert(stats, qt.IsNotNil)
		c.Assert(stats.LocationCount, qt.Equals, 1)
		c.Assert(stats.ErrorCount, qt.Equals, 0)
		c.Assert(stats.CreatedCount, qt.Equals, 1)

		// Verify no data was actually created (dry run)
		locations, err := testRegistrySet.LocationRegistry.List(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(locations), qt.Equals, 0) // Should be empty in dry run
	})

	t.Run("invalid strategy", func(t *testing.T) {
		c := qt.New(t)

		// Create test registry set
		registrySet := memory.NewRegistrySet()

		// Create restore processor
		entityService := services.NewEntityService(registrySet, "/tmp/test-uploads")
		proc := processor.NewRestoreOperationProcessor("test-op", registrySet, entityService, "/tmp/test-uploads")
		c.Assert(proc, qt.IsNotNil)

		xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
</inventory>`

		options := types.RestoreOptions{
			Strategy:        "invalid_strategy",
			IncludeFileData: false,
			DryRun:          false,
		}

		_, err := proc.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
		c.Assert(err, qt.ErrorMatches, ".*invalid restore strategy.*")
	})
}

func TestRestoreService_MainCurrencyValidation(t *testing.T) {
	c := qt.New(t)

	// Create test registries
	registrySet := memory.NewRegistrySet()

	// Set up main currency in settings
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})

	err := registrySet.SettingsRegistry.Patch(ctx, "system.main_currency", "USD")
	c.Assert(err, qt.IsNil)

	entityService := services.NewEntityService(registrySet, "")
	proc := processor.NewRestoreOperationProcessor("test-op", registrySet, entityService, "")

	// Create XML with a commodity that has pricing information
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/schema" exportDate="2023-01-01T00:00:00Z" exportType="full">
  <locations>
    <location id="loc1">
      <locationName>Test Location</locationName>
      <address>123 Test St</address>
    </location>
  </locations>
  <areas>
    <area id="area1">
      <areaName>Test Area</areaName>
      <locationId>loc1</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="comm1">
      <commodityName>Test Commodity</commodityName>
      <shortName>TC</shortName>
      <areaId>area1</areaId>
      <count>1</count>
      <status>in_use</status>
      <type>electronics</type>
      <originalPrice>100.00</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <convertedOriginalPrice>0</convertedOriginalPrice>
      <currentPrice>95.00</currentPrice>
      <draft>false</draft>
      <purchaseDate>2023-01-01</purchaseDate>
    </commodity>
  </commodities>
</inventory>`

	reader := strings.NewReader(xmlContent)
	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		IncludeFileData: false,
		DryRun:          false,
	}

	stats, err := proc.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("Expected no errors, but got: %v", stats.Errors))
	c.Assert(stats.CommodityCount, qt.Equals, 1)
	c.Assert(stats.CreatedCount, qt.Equals, 3) // 1 location + 1 area + 1 commodity

	// Verify the commodity was created successfully
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)

	commodity := commodities[0]
	c.Assert(commodity.Name, qt.Equals, "Test Commodity")
	c.Assert(commodity.OriginalPrice.String(), qt.Equals, "100")
	c.Assert(string(commodity.OriginalPriceCurrency), qt.Equals, "USD")
}

func TestRestoreService_NoMainCurrencySet(t *testing.T) {
	c := qt.New(t)

	// Create test registries without setting main currency
	registrySet := memory.NewRegistrySet()

	entityService := services.NewEntityService(registrySet, "")
	proc := processor.NewRestoreOperationProcessor("test-op", registrySet, entityService, "")

	// Create XML with a commodity that has pricing information
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/schema" exportDate="2023-01-01T00:00:00Z" exportType="full">
  <locations>
    <location id="loc1">
      <locationName>Test Location</locationName>
      <address>123 Test St</address>
    </location>
  </locations>
  <areas>
    <area id="area1">
      <areaName>Test Area</areaName>
      <locationId>loc1</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="comm1">
      <commodityName>Test Commodity</commodityName>
      <shortName>TC</shortName>
      <areaId>area1</areaId>
      <count>1</count>
      <status>in_use</status>
      <type>electronics</type>
      <originalPrice>100.00</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <convertedOriginalPrice>0</convertedOriginalPrice>
      <currentPrice>95.00</currentPrice>
      <draft>false</draft>
      <purchaseDate>2023-01-01</purchaseDate>
    </commodity>
  </commodities>
</inventory>`

	reader := strings.NewReader(xmlContent)
	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		IncludeFileData: false,
		DryRun:          false,
	}

	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	stats, err := proc.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)

	// Should have errors because main currency is not set
	c.Assert(stats.ErrorCount, qt.Equals, 1)
	c.Assert(stats.Errors, qt.HasLen, 1)
	c.Assert(stats.Errors[0], qt.Matches, ".*main currency not set.*")

	// Location and area should be created successfully
	c.Assert(stats.LocationCount, qt.Equals, 1)
	c.Assert(stats.AreaCount, qt.Equals, 1)
	c.Assert(stats.CommodityCount, qt.Equals, 0) // Commodity should fail
	c.Assert(stats.CreatedCount, qt.Equals, 2)   // Only location + area

	// Verify no commodity was created
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 0)
}

func TestRestoreService_SampleXMLStructure(t *testing.T) {
	c := qt.New(t)

	// Create test registries
	registrySet := memory.NewRegistrySet()

	// Set up main currency in settings
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})

	err := registrySet.SettingsRegistry.Patch(ctx, "system.main_currency", "CZK")
	c.Assert(err, qt.IsNil)

	entityService := services.NewEntityService(registrySet, "")
	proc := processor.NewRestoreOperationProcessor("test-op", registrySet, entityService, "")

	// Create XML with the same structure as sample_export.xml
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<inventory exportDate="2025-06-10T21:56:40Z" exportType="full_database">
  <locations>
    <location id="c1a42b71-f06e-4b93-b246-1e99c5d732a1">
      <locationName>Home</locationName>
      <address>123 Main St, Anytown, USA</address>
    </location>
  </locations>
  <areas>
    <area id="bbbb2ece-f73c-4de0-adf1-d151d872aae6">
      <areaName>Living Room</areaName>
      <locationId>c1a42b71-f06e-4b93-b246-1e99c5d732a1</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="67b0dee7-e5c0-4c1f-a216-a350072ea031">
      <commodityName>Smart TV</commodityName>
      <shortName>TV</shortName>
      <type>electronics</type>
      <areaId>bbbb2ece-f73c-4de0-adf1-d151d872aae6</areaId>
      <count>1</count>
      <originalPrice>1299.99</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <convertedOriginalPrice>29899.77</convertedOriginalPrice>
      <currentPrice>899.99</currentPrice>
      <serialNumber>TV123456789</serialNumber>
      <extraSerialNumbers></extraSerialNumbers>
      <partNumbers></partNumbers>
      <tags>
        <tag>electronics</tag>
        <tag>entertainment</tag>
      </tags>
      <status>in_use</status>
      <purchaseDate>2022-01-15</purchaseDate>
      <registeredDate>2022-01-16</registeredDate>
      <urls></urls>
      <comments>65-inch 4K Smart TV</comments>
      <draft>false</draft>
    </commodity>
    <commodity id="1350bb48-d608-425f-bd02-d9695836d3c5">
      <commodityName>Winter Clothes</commodityName>
      <shortName>Winter</shortName>
      <type>clothes</type>
      <areaId>bbbb2ece-f73c-4de0-adf1-d151d872aae6</areaId>
      <count>10</count>
      <originalPrice>1200</originalPrice>
      <originalPriceCurrency>CZK</originalPriceCurrency>
      <convertedOriginalPrice>0</convertedOriginalPrice>
      <currentPrice>600</currentPrice>
      <extraSerialNumbers></extraSerialNumbers>
      <partNumbers></partNumbers>
      <tags>
        <tag>clothes</tag>
        <tag>seasonal</tag>
      </tags>
      <status>in_use</status>
      <purchaseDate>2021-09-15</purchaseDate>
      <registeredDate>2021-09-20</registeredDate>
      <urls></urls>
      <comments>Winter clothes in storage</comments>
      <draft>false</draft>
    </commodity>
  </commodities>
</inventory>`

	reader := strings.NewReader(xmlContent)
	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		IncludeFileData: false,
		DryRun:          false,
	}

	stats, err := proc.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)

	// Debug: print errors if any
	if stats.ErrorCount > 0 {
		for _, errMsg := range stats.Errors {
			c.Logf("Error: %s", errMsg)
		}
	}

	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("Expected no errors, but got: %v", stats.Errors))
	c.Assert(stats.CommodityCount, qt.Equals, 2)
	c.Assert(stats.CreatedCount, qt.Equals, 4) // 1 location + 1 area + 2 commodities

	// Verify the commodities were created successfully
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 2)

	// Check the first commodity (Smart TV)
	var smartTV *models.Commodity
	for _, commodity := range commodities {
		if commodity.Name == "Smart TV" {
			smartTV = commodity
			break
		}
	}
	c.Assert(smartTV, qt.IsNotNil)
	c.Assert(smartTV.OriginalPrice.String(), qt.Equals, "1299.99")
	c.Assert(string(smartTV.OriginalPriceCurrency), qt.Equals, "USD")
	c.Assert(smartTV.ConvertedOriginalPrice.String(), qt.Equals, "29899.77")
	c.Assert(smartTV.SerialNumber, qt.Equals, "TV123456789")

	// Check the second commodity (Winter Clothes)
	var winterClothes *models.Commodity
	for _, commodity := range commodities {
		if commodity.Name == "Winter Clothes" {
			winterClothes = commodity
			break
		}
	}
	c.Assert(winterClothes, qt.IsNotNil)
	c.Assert(winterClothes.OriginalPrice.String(), qt.Equals, "1200")
	c.Assert(string(winterClothes.OriginalPriceCurrency), qt.Equals, "CZK")
	c.Assert(winterClothes.ConvertedOriginalPrice.String(), qt.Equals, "0")
}

func TestRestoreService_ActualSampleXML(t *testing.T) {
	c := qt.New(t)

	// Create test registries
	registrySet := memory.NewRegistrySet()

	// Set up main currency in settings
	ctx := c.Context()
	err := registrySet.SettingsRegistry.Patch(ctx, "system.main_currency", "CZK")
	c.Assert(err, qt.IsNil)

	entityService := services.NewEntityService(registrySet, "")
	proc := processor.NewRestoreOperationProcessor("test-op", registrySet, entityService, "")

	// Read the actual sample XML file
	xmlContent, err := os.ReadFile("testdata/sample_export.xml")
	c.Assert(err, qt.IsNil)

	reader := strings.NewReader(string(xmlContent))
	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		IncludeFileData: false,
		DryRun:          false,
	}

	stats, err := proc.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)

	// Debug: print errors if any
	if stats.ErrorCount > 0 {
		c.Logf("Total errors: %d", stats.ErrorCount)
		for i, errMsg := range stats.Errors {
			c.Logf("Error %d: %s", i+1, errMsg)
		}
	}

	c.Logf("Stats: LocationCount=%d, AreaCount=%d, CommodityCount=%d, CreatedCount=%d, ErrorCount=%d",
		stats.LocationCount, stats.AreaCount, stats.CommodityCount, stats.CreatedCount, stats.ErrorCount)
}
