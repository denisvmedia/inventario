package commonsql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestLocationRegistry_Create_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name     string
		location models.Location
	}{
		{
			name: "valid location with all fields",
			location: models.Location{
				Name:    "Main Office",
				Address: "123 Business Street",
			},
		},
		{
			name: "valid location with minimal fields",
			location: models.Location{
				Name:    "Warehouse",
				Address: "456 Storage Ave",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			result, err := registrySet.LocationRegistry.Create(ctx, tc.location)
			c.Assert(err, qt.IsNil)
			c.Assert(result, qt.IsNotNil)
			c.Assert(result.ID, qt.Not(qt.Equals), "")
			c.Assert(result.Name, qt.Equals, tc.location.Name)
			c.Assert(result.Address, qt.Equals, tc.location.Address)
		})
	}
}

func TestLocationRegistry_Create_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name     string
		location models.Location
	}{
		{
			name: "empty name",
			location: models.Location{
				Name:    "",
				Address: "123 Test Street",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			result, err := registrySet.LocationRegistry.Create(ctx, tc.location)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestLocationRegistry_Get_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Create a test location
	created := createTestLocation(c, registrySet.LocationRegistry)

	// Get the location
	result, err := registrySet.LocationRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.Name, qt.Equals, created.Name)
	c.Assert(result.Address, qt.Equals, created.Address)
}

func TestLocationRegistry_Get_UnhappyPath(t *testing.T) {
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
			ctx := context.Background()

			result, err := registrySet.LocationRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestLocationRegistry_List_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Initially should be empty
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations), qt.Equals, 0)

	// Create test locations
	location1 := createTestLocation(c, registrySet.LocationRegistry)
	location2 := createTestLocation(c, registrySet.LocationRegistry)

	// List should now contain both locations
	locations, err = registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations), qt.Equals, 2)

	// Verify the locations are in the list
	locationIDs := make(map[string]bool)
	for _, loc := range locations {
		locationIDs[loc.ID] = true
	}
	c.Assert(locationIDs[location1.ID], qt.IsTrue)
	c.Assert(locationIDs[location2.ID], qt.IsTrue)
}

func TestLocationRegistry_Update_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Create a test location
	created := createTestLocation(c, registrySet.LocationRegistry)

	// Update the location
	created.Name = "Updated Location"
	created.Address = "Updated Address"

	result, err := registrySet.LocationRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.Name, qt.Equals, "Updated Location")
	c.Assert(result.Address, qt.Equals, "Updated Address")

	// Verify the update persisted
	retrieved, err := registrySet.LocationRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.Name, qt.Equals, "Updated Location")
	c.Assert(retrieved.Address, qt.Equals, "Updated Address")
}

func TestLocationRegistry_Update_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name     string
		location models.Location
	}{
		{
			name: "non-existent location",
			location: models.Location{
				EntityID: models.EntityID{ID: "non-existent-id"},
				Name:     "Test Location",
				Address:  "Test Address",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			result, err := registrySet.LocationRegistry.Update(ctx, tc.location)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestLocationRegistry_Delete_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Create a test location
	created := createTestLocation(c, registrySet.LocationRegistry)

	// Delete the location
	err := registrySet.LocationRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify the location is deleted
	result, err := registrySet.LocationRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
}

func TestLocationRegistry_Delete_UnhappyPath(t *testing.T) {
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
			ctx := context.Background()

			err := registrySet.LocationRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestLocationRegistry_Delete_WithAreas_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Create a test location
	location := createTestLocation(c, registrySet.LocationRegistry)

	// Create an area in the location
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)

	// Add area to location
	err := registrySet.LocationRegistry.AddArea(ctx, location.ID, area.ID)
	c.Assert(err, qt.IsNil)

	// Try to delete the location - should fail because it has areas
	err = registrySet.LocationRegistry.Delete(ctx, location.ID)
	c.Assert(err, qt.IsNotNil)

	// Verify the location still exists
	result, err := registrySet.LocationRegistry.Get(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
}

func TestLocationRegistry_Count_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Initially should be 0
	count, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test locations
	createTestLocation(c, registrySet.LocationRegistry)
	createTestLocation(c, registrySet.LocationRegistry)

	// Count should now be 2
	count, err = registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

func TestLocationRegistry_AddArea_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Create a test location and area
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)

	// Add area to location
	err := registrySet.LocationRegistry.AddArea(ctx, location.ID, area.ID)
	c.Assert(err, qt.IsNil)

	// Verify the area is added
	areas, err := registrySet.LocationRegistry.GetAreas(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 1)
	c.Assert(areas[0], qt.Equals, area.ID)
}

func TestLocationRegistry_AddArea_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name       string
		locationID string
		areaID     string
	}{
		{
			name:       "non-existent location",
			locationID: "non-existent-location",
			areaID:     "some-area-id",
		},
		{
			name:       "non-existent area",
			locationID: "some-location-id",
			areaID:     "non-existent-area",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			err := registrySet.LocationRegistry.AddArea(ctx, tc.locationID, tc.areaID)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestLocationRegistry_GetAreas_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Create a test location
	location := createTestLocation(c, registrySet.LocationRegistry)

	// Initially should have no areas
	areas, err := registrySet.LocationRegistry.GetAreas(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 0)

	// Create and add areas
	area1 := createTestArea(c, registrySet.AreaRegistry, location.ID)
	area2 := createTestArea(c, registrySet.AreaRegistry, location.ID)

	err = registrySet.LocationRegistry.AddArea(ctx, location.ID, area1.ID)
	c.Assert(err, qt.IsNil)
	err = registrySet.LocationRegistry.AddArea(ctx, location.ID, area2.ID)
	c.Assert(err, qt.IsNil)

	// Should now have 2 areas
	areas, err = registrySet.LocationRegistry.GetAreas(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 2)

	// Verify the area IDs are correct
	areaIDs := make(map[string]bool)
	for _, areaID := range areas {
		areaIDs[areaID] = true
	}
	c.Assert(areaIDs[area1.ID], qt.IsTrue)
	c.Assert(areaIDs[area2.ID], qt.IsTrue)
}

func TestLocationRegistry_GetAreas_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name       string
		locationID string
	}{
		{
			name:       "non-existent location",
			locationID: "non-existent-location",
		},
		{
			name:       "empty location ID",
			locationID: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			areas, err := registrySet.LocationRegistry.GetAreas(ctx, tc.locationID)
			c.Assert(err, qt.IsNotNil)
			c.Assert(areas, qt.IsNil)
		})
	}
}

func TestLocationRegistry_DeleteArea_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Create a test location and area
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)

	// Add area to location
	err := registrySet.LocationRegistry.AddArea(ctx, location.ID, area.ID)
	c.Assert(err, qt.IsNil)

	// Verify the area is added
	areas, err := registrySet.LocationRegistry.GetAreas(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 1)

	// Delete the area from location
	err = registrySet.LocationRegistry.DeleteArea(ctx, location.ID, area.ID)
	c.Assert(err, qt.IsNil)

	// Verify the area is removed from location but still exists
	areas, err = registrySet.LocationRegistry.GetAreas(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 0)

	// Verify the area itself is deleted (based on the implementation)
	result, err := registrySet.AreaRegistry.Get(ctx, area.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
}

func TestLocationRegistry_DeleteArea_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name       string
		locationID string
		areaID     string
	}{
		{
			name:       "non-existent location",
			locationID: "non-existent-location",
			areaID:     "some-area-id",
		},
		{
			name:       "non-existent area",
			locationID: "some-location-id",
			areaID:     "non-existent-area",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			err := registrySet.LocationRegistry.DeleteArea(ctx, tc.locationID, tc.areaID)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestLocationRegistry_DeleteArea_AreaNotInLocation_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	// Create two locations and an area in the first location
	location1 := createTestLocation(c, registrySet.LocationRegistry)
	location2 := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location1.ID)

	// Add area to location1
	err := registrySet.LocationRegistry.AddArea(ctx, location1.ID, area.ID)
	c.Assert(err, qt.IsNil)

	// Try to delete the area from location2 - should fail
	err = registrySet.LocationRegistry.DeleteArea(ctx, location2.ID, area.ID)
	c.Assert(err, qt.IsNotNil)

	// Verify the area is still in location1
	areas, err := registrySet.LocationRegistry.GetAreas(ctx, location1.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 1)
	c.Assert(areas[0], qt.Equals, area.ID)
}
