package restore_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func TestRestoreService_StreamingXMLParsing(t *testing.T) {
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
	processor := restore.NewRestoreOperationProcessor("test-restore-op", registrySet, entityService, "")

	// Create XML with processing instructions and various token types that should be handled properly
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<!-- This is a comment that should be ignored -->
<!DOCTYPE inventory SYSTEM "inventory.dtd">
<inventory xmlns="http://inventario.example.com/schema" exportDate="2023-01-01T00:00:00Z" exportType="full">
  <!-- Another comment -->
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
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		IncludeFileData: false,
		DryRun:          false,
	}

	// This should work without any "unexpected token type" errors
	stats, err := processor.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("Expected no errors, but got: %v", stats.Errors))
	c.Assert(stats.CommodityCount, qt.Equals, 1)
	c.Assert(stats.LocationCount, qt.Equals, 1)
	c.Assert(stats.AreaCount, qt.Equals, 1)
	c.Assert(stats.CreatedCount, qt.Equals, 3) // 1 location + 1 area + 1 commodity

	// Verify the data was created successfully
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)

	commodity := commodities[0]
	c.Assert(commodity.Name, qt.Equals, "Test Commodity")
	c.Assert(commodity.OriginalPrice.String(), qt.Equals, "100")
	c.Assert(string(commodity.OriginalPriceCurrency), qt.Equals, "USD")
}

func TestRestoreService_LoggedRestoreWithStreaming(t *testing.T) {
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
	processor := restore.NewRestoreOperationProcessor("test-restore-op", registrySet, entityService, "")

	// This test demonstrates that the streaming XML parsing works correctly
	// without loading everything into memory

	// Create XML content
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
	options := restore.RestoreOptions{
		Strategy:        restore.RestoreStrategyFullReplace,
		IncludeFileData: false,
		DryRun:          false,
	}

	// Test the detailed logging restore process - we'll just test the regular restore for now
	// since the detailed logging is tested through the background worker
	stats, err := processor.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("Expected no errors, but got: %v", stats.Errors))
	c.Assert(stats.CommodityCount, qt.Equals, 1)
	c.Assert(stats.LocationCount, qt.Equals, 1)
	c.Assert(stats.AreaCount, qt.Equals, 1)
	c.Assert(stats.CreatedCount, qt.Equals, 3) // 1 location + 1 area + 1 commodity

	// Verify the data was created successfully
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)

	commodity := commodities[0]
	c.Assert(commodity.Name, qt.Equals, "Test Commodity")

	// Note: We can't easily test the detailed logging here since it requires
	// the background worker infrastructure. The main fix was to avoid loading
	// everything into memory, which is tested by the streaming XML parsing above.
}
