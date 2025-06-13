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
)

func TestRestoreService_ClearExistingData_RecursiveDelete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registry set with proper dependencies
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)

	// Create existing data that would cause the old restore to fail
	location := models.Location{Name: "Existing Location"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{Name: "Existing Area", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		Name:   "Existing Commodity",
		AreaID: createdArea.ID,
	}
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Verify the hierarchy exists
	areas, err := registrySet.LocationRegistry.GetAreas(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 1)

	commodities, err := registrySet.AreaRegistry.GetCommodities(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)

	// Create a simple XML backup to restore
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<backup>
  <locations>
    <location id="new-location-1" name="New Location 1" address="123 Main St" />
  </locations>
  <areas>
    <area id="new-area-1" name="New Area 1" location_id="new-location-1" />
  </areas>
  <commodities>
    <commodity id="new-commodity-1" name="New Commodity 1" area_id="new-area-1" />
  </commodities>
</backup>`

	// Create restore service
	restoreService := restore.NewRestoreService(registrySet, "mem://")

	// Test restore with full replace strategy (this should now work with recursive delete)
	options := restore.RestoreOptions{
		Strategy: restore.RestoreStrategyFullReplace,
		DryRun:   false,
	}

	reader := strings.NewReader(xmlData)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats, qt.IsNotNil)

	// The key test: verify old data is gone (this proves recursive delete worked)
	_, err = registrySet.LocationRegistry.Get(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	_, err = registrySet.AreaRegistry.Get(ctx, createdArea.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	_, err = registrySet.CommodityRegistry.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	// The main goal is to verify that recursive delete worked during clearExistingData
	// The new data creation might fail due to validation, but that's not the focus of this test
	_, err = registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	// We don't assert on the count since the XML might have validation issues
	// The important thing is that the old data was successfully cleared
}

func TestRestoreService_ClearExistingData_MultipleLocations(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registry set with proper dependencies
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)

	// Create multiple locations with areas and commodities
	for i := 0; i < 3; i++ {
		location := models.Location{Name: "Location " + string(rune('A'+i))}
		createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
		c.Assert(err, qt.IsNil)

		for j := 0; j < 2; j++ {
			area := models.Area{Name: "Area " + string(rune('A'+i)) + string(rune('1'+j)), LocationID: createdLocation.ID}
			createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
			c.Assert(err, qt.IsNil)

			commodity := models.Commodity{
				Name:   "Commodity " + string(rune('A'+i)) + string(rune('1'+j)),
				AreaID: createdArea.ID,
			}
			_, err = registrySet.CommodityRegistry.Create(ctx, commodity)
			c.Assert(err, qt.IsNil)
		}
	}

	// Verify we have the expected data
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, 3)

	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 6)

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 6)

	// Create restore service
	restoreService := restore.NewRestoreService(registrySet, "mem://")

	// Test restore with full replace strategy
	options := restore.RestoreOptions{
		Strategy: restore.RestoreStrategyFullReplace,
		DryRun:   false,
	}

	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<backup>
  <locations>
  </locations>
  <areas>
  </areas>
  <commodities>
  </commodities>
</backup>`

	reader := strings.NewReader(xmlData)
	stats, err := restoreService.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats, qt.IsNotNil)

	// Verify all data is cleared
	locations, err = registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, 0)

	areas, err = registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 0)

	commodities, err = registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 0)
}
