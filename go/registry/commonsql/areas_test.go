package commonsql_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestAreaRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name string
		area models.Area
	}{
		{
			name: "valid area with all fields",
			area: models.Area{
				Name: "Main Storage",
			},
		},
		{
			name: "valid area with minimal fields",
			area: models.Area{
				Name: "Secondary Storage",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Create a test location first
			location := createTestLocation(c, registrySet.LocationRegistry)
			tc.area.LocationID = location.GetID()

			// Create area
			result, err := registrySet.AreaRegistry.Create(ctx, tc.area)
			c.Assert(err, qt.IsNil)
			c.Assert(result, qt.IsNotNil)
			c.Assert(result.ID, qt.Not(qt.Equals), "")
			c.Assert(result.Name, qt.Equals, tc.area.Name)
			c.Assert(result.LocationID, qt.Equals, tc.area.LocationID)
		})
	}
}

func TestAreaRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name string
		area models.Area
	}{
		{
			name: "empty name",
			area: models.Area{
				Name: "",
			},
		},
		{
			name: "empty location ID",
			area: models.Area{
				Name:       "Test Area",
				LocationID: "",
			},
		},
		{
			name: "non-existent location",
			area: models.Area{
				Name:       "Test Area",
				LocationID: "non-existent-location",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For the non-existent location test, we don't need to create a location
			// For other tests, create a location if LocationID is not empty
			if tc.area.LocationID != "" && tc.area.LocationID != "non-existent-location" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				tc.area.LocationID = location.GetID()
			}

			result, err := registrySet.AreaRegistry.Create(ctx, tc.area)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestAreaRegistry_Get_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create a test location and area
	location := createTestLocation(c, registrySet.LocationRegistry)
	created := createTestArea(c, registrySet.AreaRegistry, location.ID)

	// Get the area
	result, err := registrySet.AreaRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.Name, qt.Equals, created.Name)
	c.Assert(result.LocationID, qt.Equals, created.LocationID)
}

func TestAreaRegistry_Get_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent ID",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			result, err := registrySet.AreaRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestAreaRegistry_List_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Initially should be empty
	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 0)

	// Create test location and areas
	location := createTestLocation(c, registrySet.LocationRegistry)
	area1 := createTestArea(c, registrySet.AreaRegistry, location.ID)
	area2 := createTestArea(c, registrySet.AreaRegistry, location.ID)

	// List should now contain both areas
	areas, err = registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 2)

	// Verify the areas are in the list
	areaIDs := make(map[string]bool)
	for _, area := range areas {
		areaIDs[area.ID] = true
	}
	c.Assert(areaIDs[area1.ID], qt.IsTrue)
	c.Assert(areaIDs[area2.ID], qt.IsTrue)
}

func TestAreaRegistry_Update_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test location and area
	location := createTestLocation(c, registrySet.LocationRegistry)
	created := createTestArea(c, registrySet.AreaRegistry, location.ID)

	// Update the area
	created.Name = "Updated Area"

	result, err := registrySet.AreaRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.Name, qt.Equals, "Updated Area")
	c.Assert(result.LocationID, qt.Equals, created.LocationID)

	// Verify the update persisted
	retrieved, err := registrySet.AreaRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.Name, qt.Equals, "Updated Area")
	c.Assert(retrieved.LocationID, qt.Equals, created.LocationID)
}

func TestAreaRegistry_Update_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		area models.Area
	}{
		{
			name: "non-existent area",
			area: models.Area{
				TenantAwareEntityID: models.WithTenantAwareEntityID("non-existent-id", "default-tenant"),
				Name:       "Test Area",
				LocationID: "some-location-id",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			result, err := registrySet.AreaRegistry.Update(ctx, tc.area)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestAreaRegistry_Delete_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test location and area
	location := createTestLocation(c, registrySet.LocationRegistry)
	created := createTestArea(c, registrySet.AreaRegistry, location.ID)

	// Delete the area
	err := registrySet.AreaRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify the area is deleted
	result, err := registrySet.AreaRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
}

func TestAreaRegistry_Delete_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent ID",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			err := registrySet.AreaRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestAreaRegistry_Delete_WithCommodities_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)

	// Add commodity to area
	err := registrySet.AreaRegistry.AddCommodity(ctx, area.ID, commodity.ID)
	c.Assert(err, qt.IsNil)

	// Try to delete the area - should fail because it has commodities
	err = registrySet.AreaRegistry.Delete(ctx, area.ID)
	c.Assert(err, qt.IsNotNil)

	// Verify the area still exists
	result, err := registrySet.AreaRegistry.Get(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
}

func TestAreaRegistry_Count_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Initially should be 0
	count, err := registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test location and areas
	location := createTestLocation(c, registrySet.LocationRegistry)
	createTestArea(c, registrySet.AreaRegistry, location.ID)
	createTestArea(c, registrySet.AreaRegistry, location.ID)

	// Count should now be 2
	count, err = registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

func TestAreaRegistry_AddCommodity_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)

	// Add commodity to area
	err := registrySet.AreaRegistry.AddCommodity(ctx, area.ID, commodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify the commodity is added
	commodities, err := registrySet.AreaRegistry.GetCommodities(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)
	c.Assert(commodities[0], qt.Equals, commodity.ID)
}

func TestAreaRegistry_AddCommodity_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name        string
		areaID      string
		commodityID string
	}{
		{
			name:        "non-existent area",
			areaID:      "non-existent-area",
			commodityID: "some-commodity-id",
		},
		{
			name:        "non-existent commodity",
			areaID:      "some-area-id",
			commodityID: "non-existent-commodity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			err := registrySet.AreaRegistry.AddCommodity(ctx, tc.areaID, tc.commodityID)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestAreaRegistry_GetCommodities_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)

	// Initially should have no commodities
	commodities, err := registrySet.AreaRegistry.GetCommodities(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 0)

	// Create and add commodities
	commodity1 := createTestCommodity(c, registrySet, area.ID)
	commodity2 := createTestCommodity(c, registrySet, area.ID)

	err = registrySet.AreaRegistry.AddCommodity(ctx, area.ID, commodity1.ID)
	c.Assert(err, qt.IsNil)
	err = registrySet.AreaRegistry.AddCommodity(ctx, area.ID, commodity2.ID)
	c.Assert(err, qt.IsNil)

	// Should now have 2 commodities
	commodities, err = registrySet.AreaRegistry.GetCommodities(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 2)

	// Verify the commodity IDs are correct
	commodityIDs := make(map[string]bool)
	for _, commodityID := range commodities {
		commodityIDs[commodityID] = true
	}
	c.Assert(commodityIDs[commodity1.ID], qt.IsTrue)
	c.Assert(commodityIDs[commodity2.ID], qt.IsTrue)
}

func TestAreaRegistry_GetCommodities_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name   string
		areaID string
	}{
		{
			name:   "non-existent area",
			areaID: "non-existent-area",
		},
		{
			name:   "empty area ID",
			areaID: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			commodities, err := registrySet.AreaRegistry.GetCommodities(ctx, tc.areaID)
			c.Assert(err, qt.IsNotNil)
			c.Assert(commodities, qt.IsNil)
		})
	}
}

func TestAreaRegistry_DeleteCommodity_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)

	// Add commodity to area
	err := registrySet.AreaRegistry.AddCommodity(ctx, area.ID, commodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify the commodity is added
	commodities, err := registrySet.AreaRegistry.GetCommodities(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)

	// Delete the commodity from area
	err = registrySet.AreaRegistry.DeleteCommodity(ctx, area.ID, commodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify the commodity is removed from area and deleted entirely
	commodities, err = registrySet.AreaRegistry.GetCommodities(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 0)

	// Verify the commodity itself is deleted
	result, err := registrySet.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
}

func TestAreaRegistry_DeleteCommodity_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name        string
		areaID      string
		commodityID string
	}{
		{
			name:        "non-existent area",
			areaID:      "non-existent-area",
			commodityID: "some-commodity-id",
		},
		{
			name:        "non-existent commodity",
			areaID:      "some-area-id",
			commodityID: "non-existent-commodity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			err := registrySet.AreaRegistry.DeleteCommodity(ctx, tc.areaID, tc.commodityID)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestAreaRegistry_DeleteCommodity_CommodityNotInArea_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create two areas and a commodity in the first area
	location := createTestLocation(c, registrySet.LocationRegistry)
	area1 := createTestArea(c, registrySet.AreaRegistry, location.ID)
	area2 := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area1.ID)

	// Add commodity to area1
	err := registrySet.AreaRegistry.AddCommodity(ctx, area1.ID, commodity.ID)
	c.Assert(err, qt.IsNil)

	// Try to delete the commodity from area2 - should fail
	err = registrySet.AreaRegistry.DeleteCommodity(ctx, area2.ID, commodity.ID)
	c.Assert(err, qt.IsNotNil)

	// Verify the commodity is still in area1
	commodities, err := registrySet.AreaRegistry.GetCommodities(ctx, area1.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)
	c.Assert(commodities[0], qt.Equals, commodity.ID)
}
