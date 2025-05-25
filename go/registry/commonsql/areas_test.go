package commonsql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// TestAreaRegistry_Create_HappyPath tests successful area creation scenarios.
func TestAreaRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name string
		area models.Area
	}{
		{
			name: "basic area",
			area: models.Area{
				Name: "Test Area",
			},
		},
		{
			name: "area with special characters",
			area: models.Area{
				Name: "Área de Almacén #1",
			},
		},
		{
			name: "area with long name",
			area: models.Area{
				Name: "Very Long Area Name That Goes On And On And Contains Many Words",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Create a test location first
			location := createTestLocation(c, registrySet.LocationRegistry)
			tc.area.LocationID = location.GetID()

			// Create area
			createdArea, err := registrySet.AreaRegistry.Create(ctx, tc.area)
			c.Assert(err, qt.IsNil)
			c.Assert(createdArea, qt.IsNotNil)
			c.Assert(createdArea.GetID(), qt.Not(qt.Equals), "")
			c.Assert(createdArea.Name, qt.Equals, tc.area.Name)
			c.Assert(createdArea.LocationID, qt.Equals, tc.area.LocationID)

			// Verify count
			count, err := registrySet.AreaRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 1)
		})
	}
}

// TestAreaRegistry_Create_UnhappyPath tests area creation error scenarios.
func TestAreaRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name string
		area models.Area
	}{
		{
			name: "empty name",
			area: models.Area{
				Name:       "",
				LocationID: "some-location-id",
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
			name: "both empty",
			area: models.Area{
				Name:       "",
				LocationID: "",
			},
		},
		{
			name: "non-existent location ID",
			area: models.Area{
				Name:       "Test Area",
				LocationID: "non-existent-location",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For the non-existent location test, we don't need to create a location
			// For other tests, create a location if LocationID is not empty
			if tc.area.LocationID != "" && tc.area.LocationID != "non-existent-location" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				tc.area.LocationID = location.GetID()
			}

			// Attempt to create invalid area
			_, err := registrySet.AreaRegistry.Create(ctx, tc.area)
			c.Assert(err, qt.IsNotNil)

			// Verify count remains zero
			count, err := registrySet.AreaRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 0)
		})
	}
}

// TestAreaRegistry_Get_HappyPath tests successful area retrieval scenarios.
func TestAreaRegistry_Get_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	// Get the area
	retrievedArea, err := registrySet.AreaRegistry.Get(ctx, area.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedArea, qt.IsNotNil)
	c.Assert(retrievedArea.GetID(), qt.Equals, area.GetID())
	c.Assert(retrievedArea.Name, qt.Equals, area.Name)
	c.Assert(retrievedArea.LocationID, qt.Equals, area.LocationID)
}

// TestAreaRegistry_Get_UnhappyPath tests area retrieval error scenarios.
func TestAreaRegistry_Get_UnhappyPath(t *testing.T) {
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
		{
			name: "UUID format but non-existent",
			id:   "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to get non-existent area
			_, err := registrySet.AreaRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorMatches, ".*not found.*")
		})
	}
}

// TestAreaRegistry_Update_HappyPath tests successful area update scenarios.
func TestAreaRegistry_Update_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	// Update the area
	area.Name = "Updated Area"

	updatedArea, err := registrySet.AreaRegistry.Update(ctx, *area)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedArea, qt.IsNotNil)
	c.Assert(updatedArea.GetID(), qt.Equals, area.GetID())
	c.Assert(updatedArea.Name, qt.Equals, "Updated Area")
	c.Assert(updatedArea.LocationID, qt.Equals, area.LocationID)

	// Verify the update persisted
	retrievedArea, err := registrySet.AreaRegistry.Get(ctx, area.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedArea.Name, qt.Equals, "Updated Area")
}

// TestAreaRegistry_Update_UnhappyPath tests area update error scenarios.
func TestAreaRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name string
		area models.Area
	}{
		{
			name: "non-existent area",
			area: models.Area{
				ID:         "non-existent-id",
				Name:       "Test Area",
				LocationID: "some-location-id",
			},
		},
		{
			name: "empty name",
			area: models.Area{
				ID:         "some-id",
				Name:       "",
				LocationID: "some-location-id",
			},
		},
		{
			name: "empty location ID",
			area: models.Area{
				ID:         "some-id",
				Name:       "Test Area",
				LocationID: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to update with invalid data
			_, err := registrySet.AreaRegistry.Update(ctx, tc.area)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestAreaRegistry_Delete_HappyPath tests successful area deletion scenarios.
func TestAreaRegistry_Delete_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	// Verify area exists
	_, err := registrySet.AreaRegistry.Get(ctx, area.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the area
	err = registrySet.AreaRegistry.Delete(ctx, area.GetID())
	c.Assert(err, qt.IsNil)

	// Verify area is deleted
	_, err = registrySet.AreaRegistry.Get(ctx, area.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

// TestAreaRegistry_Delete_UnhappyPath tests area deletion error scenarios.
func TestAreaRegistry_Delete_UnhappyPath(t *testing.T) {
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
		{
			name: "UUID format but non-existent",
			id:   "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to delete non-existent area
			err := registrySet.AreaRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestAreaRegistry_List_HappyPath tests successful area listing scenarios.
func TestAreaRegistry_List_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty list
	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area1 := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	area2 := models.Area{
		Name:       "Second Area",
		LocationID: location.GetID(),
	}
	createdArea2, err := registrySet.AreaRegistry.Create(ctx, area2)
	c.Assert(err, qt.IsNil)

	// List all areas
	areas, err = registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 2)

	// Verify areas are in the list
	areaIDs := make(map[string]bool)
	for _, area := range areas {
		areaIDs[area.GetID()] = true
	}
	c.Assert(areaIDs[area1.GetID()], qt.IsTrue)
	c.Assert(areaIDs[createdArea2.GetID()], qt.IsTrue)
}

// TestAreaRegistry_Count_HappyPath tests successful area counting scenarios.
func TestAreaRegistry_Count_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty count
	count, err := registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	createTestArea(c, registrySet.AreaRegistry, location.GetID())

	count, err = registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)

	// Create another area
	area2 := models.Area{
		Name:       "Second Area",
		LocationID: location.GetID(),
	}
	_, err = registrySet.AreaRegistry.Create(ctx, area2)
	c.Assert(err, qt.IsNil)

	count, err = registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

// TestAreaRegistry_Commodities_HappyPath tests area-commodity relationship management.
func TestAreaRegistry_Commodities_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	// Initially no commodities
	commodities, err := registrySet.AreaRegistry.GetCommodities(ctx, area.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 0)

	// Create a commodity
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Add commodity to area
	err = registrySet.AreaRegistry.AddCommodity(ctx, area.GetID(), commodity.GetID())
	c.Assert(err, qt.IsNil)

	// Verify commodity is added
	commodities, err = registrySet.AreaRegistry.GetCommodities(ctx, area.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 1)
	c.Assert(commodities[0], qt.Equals, commodity.GetID())

	// Remove commodity from area
	err = registrySet.AreaRegistry.DeleteCommodity(ctx, area.GetID(), commodity.GetID())
	c.Assert(err, qt.IsNil)

	// Verify commodity is removed
	commodities, err = registrySet.AreaRegistry.GetCommodities(ctx, area.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 0)
}

// TestAreaRegistry_Commodities_UnhappyPath tests area-commodity relationship error scenarios.
func TestAreaRegistry_Commodities_UnhappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test with non-existent area
	err := registrySet.AreaRegistry.AddCommodity(ctx, "non-existent-area", "some-commodity")
	c.Assert(err, qt.IsNotNil)

	// Test getting commodities for non-existent area
	_, err = registrySet.AreaRegistry.GetCommodities(ctx, "non-existent-area")
	c.Assert(err, qt.IsNotNil)

	// Test deleting commodity from non-existent area
	err = registrySet.AreaRegistry.DeleteCommodity(ctx, "non-existent-area", "some-commodity")
	c.Assert(err, qt.IsNotNil)
}

// TestAreaRegistry_CascadeDelete tests that deleting a location cascades to areas.
func TestAreaRegistry_CascadeDelete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	// Verify area exists
	_, err := registrySet.AreaRegistry.Get(ctx, area.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the location (should cascade to area)
	err = registrySet.LocationRegistry.Delete(ctx, location.GetID())
	c.Assert(err, qt.IsNil)

	// Verify area is also deleted due to cascade
	_, err = registrySet.AreaRegistry.Get(ctx, area.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.AreaRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
