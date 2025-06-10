package restore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/restore"
)

func TestRestoreService_RestoreFromXML(t *testing.T) {
	c := qt.New(t)

	// Create test registry set
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Create restore service
	service := restore.NewRestoreService(registrySet, "/tmp/test-uploads")
	c.Assert(service, qt.IsNotNil)

	ctx := validationctx.WithMainCurrency(context.Background(), "USD")

	t.Run("restore XML with full replace strategy", func(t *testing.T) {
		c := qt.New(t)

		// Create fresh registry set for this test
		testRegistrySet, err := memory.NewRegistrySet("")
		c.Assert(err, qt.IsNil)
		testService := restore.NewRestoreService(testRegistrySet, "/tmp/test-uploads")

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
			<originalCurrency>USD</originalCurrency>
			<currentPrice>90.00</currentPrice>
			<purchaseDate>2023-01-01</purchaseDate>
		</commodity>
	</commodities>
</inventory>`

		options := restore.RestoreOptions{
			Strategy:        restore.RestoreStrategyFullReplace,
			IncludeFileData: false,
			DryRun:          false,
			BackupExisting:  false,
		}

		stats, err := testService.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
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
		testRegistrySet, err := memory.NewRegistrySet("")
		c.Assert(err, qt.IsNil)
		testService := restore.NewRestoreService(testRegistrySet, "/tmp/test-uploads")

		// First, create some existing data
		existingLocation := models.Location{
			EntityID: models.EntityID{ID: "loc1"},
			Name:     "Existing Location",
			Address:  "456 Existing St",
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

		options := restore.RestoreOptions{
			Strategy:        restore.RestoreStrategyMergeAdd,
			IncludeFileData: false,
			DryRun:          false,
			BackupExisting:  false,
		}

		stats, err := testService.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
		c.Assert(err, qt.IsNil)
		c.Assert(stats, qt.IsNotNil)
		c.Assert(stats.LocationCount, qt.Equals, 1) // Only new location counted
		c.Assert(stats.ErrorCount, qt.Equals, 0)
		c.Assert(stats.CreatedCount, qt.Equals, 1)  // Only new location created
		c.Assert(stats.SkippedCount, qt.Equals, 1)  // Existing location skipped

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
		testRegistrySet, err := memory.NewRegistrySet("")
		c.Assert(err, qt.IsNil)
		testService := restore.NewRestoreService(testRegistrySet, "/tmp/test-uploads")

		// First, create some existing data
		existingLocation := models.Location{
			EntityID: models.EntityID{ID: "loc1"},
			Name:     "Existing Location",
			Address:  "456 Existing St",
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

		options := restore.RestoreOptions{
			Strategy:        restore.RestoreStrategyMergeUpdate,
			IncludeFileData: false,
			DryRun:          false,
			BackupExisting:  false,
		}

		stats, err := testService.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
		c.Assert(err, qt.IsNil)
		c.Assert(stats, qt.IsNotNil)
		c.Assert(stats.LocationCount, qt.Equals, 2) // Both locations processed
		c.Assert(stats.ErrorCount, qt.Equals, 0)
		c.Assert(stats.CreatedCount, qt.Equals, 1)  // New location created
		c.Assert(stats.UpdatedCount, qt.Equals, 1)  // Existing location updated

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
		testRegistrySet, err := memory.NewRegistrySet("")
		c.Assert(err, qt.IsNil)
		testService := restore.NewRestoreService(testRegistrySet, "/tmp/test-uploads")

		xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
	<locations>
		<location id="loc1">
			<locationName>Test Location</locationName>
			<address>123 Test St</address>
		</location>
	</locations>
</inventory>`

		options := restore.RestoreOptions{
			Strategy:        restore.RestoreStrategyFullReplace,
			IncludeFileData: false,
			DryRun:          true, // Dry run mode
			BackupExisting:  false,
		}

		stats, err := testService.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
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

		xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
</inventory>`

		options := restore.RestoreOptions{
			Strategy:        "invalid_strategy",
			IncludeFileData: false,
			DryRun:          false,
			BackupExisting:  false,
		}

		_, err := service.RestoreFromXML(ctx, strings.NewReader(xmlData), options)
		c.Assert(err, qt.ErrorMatches, ".*invalid restore strategy.*")
	})
}
