package commonsql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// TestLocationRegistry_Create_HappyPath tests successful location creation scenarios.
func TestLocationRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name     string
		location models.Location
	}{
		{
			name: "basic location",
			location: models.Location{
				Name:    "Test Location",
				Address: "123 Test Street",
			},
		},
		{
			name: "location with special characters",
			location: models.Location{
				Name:    "Café & Restaurant",
				Address: "456 Ñoño Street, São Paulo",
			},
		},
		{
			name: "location with long address",
			location: models.Location{
				Name:    "Warehouse Complex",
				Address: "1234 Very Long Street Name That Goes On And On, Building A, Floor 5, Room 501, City, State, Country, Postal Code 12345",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Create location
			createdLocation, err := registrySet.LocationRegistry.Create(ctx, tc.location)
			c.Assert(err, qt.IsNil)
			c.Assert(createdLocation, qt.IsNotNil)
			c.Assert(createdLocation.GetID(), qt.Not(qt.Equals), "")
			c.Assert(createdLocation.Name, qt.Equals, tc.location.Name)
			c.Assert(createdLocation.Address, qt.Equals, tc.location.Address)

			// Verify count
			count, err := registrySet.LocationRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 1)
		})
	}
}

// TestLocationRegistry_Create_UnhappyPath tests location creation error scenarios.
func TestLocationRegistry_Create_UnhappyPath(t *testing.T) {
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
		{
			name: "empty address",
			location: models.Location{
				Name:    "Test Location",
				Address: "",
			},
		},
		{
			name: "both empty",
			location: models.Location{
				Name:    "",
				Address: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Attempt to create invalid location
			_, err := registrySet.LocationRegistry.Create(ctx, tc.location)
			c.Assert(err, qt.IsNotNil)

			// Verify count remains zero
			count, err := registrySet.LocationRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 0)
		})
	}
}

// TestLocationRegistry_Get_HappyPath tests successful location retrieval scenarios.
func TestLocationRegistry_Get_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create a test location
	location := createTestLocation(c, registrySet.LocationRegistry)

	// Get the location
	retrievedLocation, err := registrySet.LocationRegistry.Get(ctx, location.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedLocation, qt.IsNotNil)
	c.Assert(retrievedLocation.GetID(), qt.Equals, location.GetID())
	c.Assert(retrievedLocation.Name, qt.Equals, location.Name)
	c.Assert(retrievedLocation.Address, qt.Equals, location.Address)
}

// TestLocationRegistry_Get_UnhappyPath tests location retrieval error scenarios.
func TestLocationRegistry_Get_UnhappyPath(t *testing.T) {
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

			// Try to get non-existent location
			_, err := registrySet.LocationRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorMatches, ".*not found.*")
		})
	}
}

// TestLocationRegistry_Update_HappyPath tests successful location update scenarios.
func TestLocationRegistry_Update_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create a test location
	location := createTestLocation(c, registrySet.LocationRegistry)

	// Update the location
	location.Name = "Updated Location"
	location.Address = "456 Updated Street"

	updatedLocation, err := registrySet.LocationRegistry.Update(ctx, *location)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedLocation, qt.IsNotNil)
	c.Assert(updatedLocation.GetID(), qt.Equals, location.GetID())
	c.Assert(updatedLocation.Name, qt.Equals, "Updated Location")
	c.Assert(updatedLocation.Address, qt.Equals, "456 Updated Street")

	// Verify the update persisted
	retrievedLocation, err := registrySet.LocationRegistry.Get(ctx, location.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedLocation.Name, qt.Equals, "Updated Location")
	c.Assert(retrievedLocation.Address, qt.Equals, "456 Updated Street")
}

// TestLocationRegistry_Update_UnhappyPath tests location update error scenarios.
func TestLocationRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name     string
		location models.Location
	}{
		{
			name: "non-existent location",
			location: models.Location{
				EntityID: models.EntityID{ID: "non-existent-id"},
				Name:     "Test Location",
				Address:  "123 Test Street",
			},
		},
		{
			name: "empty name",
			location: models.Location{
				EntityID: models.EntityID{ID: "some-id"},
				Name:     "",
				Address:  "123 Test Street",
			},
		},
		{
			name: "empty address",
			location: models.Location{
				EntityID: models.EntityID{ID: "some-id"},
				Name:     "Test Location",
				Address:  "",
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
			_, err := registrySet.LocationRegistry.Update(ctx, tc.location)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestLocationRegistry_Delete_HappyPath tests successful location deletion scenarios.
func TestLocationRegistry_Delete_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create a test location
	location := createTestLocation(c, registrySet.LocationRegistry)

	// Verify location exists
	_, err := registrySet.LocationRegistry.Get(ctx, location.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the location
	err = registrySet.LocationRegistry.Delete(ctx, location.GetID())
	c.Assert(err, qt.IsNil)

	// Verify location is deleted
	_, err = registrySet.LocationRegistry.Get(ctx, location.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

// TestLocationRegistry_Delete_UnhappyPath tests location deletion error scenarios.
func TestLocationRegistry_Delete_UnhappyPath(t *testing.T) {
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

			// Try to delete non-existent location
			err := registrySet.LocationRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestLocationRegistry_List_HappyPath tests successful location listing scenarios.
func TestLocationRegistry_List_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty list
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, 0)

	// Create multiple locations
	location1 := createTestLocation(c, registrySet.LocationRegistry)
	location2 := models.Location{
		Name:    "Second Location",
		Address: "456 Second Street",
	}
	createdLocation2, err := registrySet.LocationRegistry.Create(ctx, location2)
	c.Assert(err, qt.IsNil)

	// List all locations
	locations, err = registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, 2)

	// Verify locations are in the list
	locationIDs := make(map[string]bool)
	for _, loc := range locations {
		locationIDs[loc.GetID()] = true
	}
	c.Assert(locationIDs[location1.GetID()], qt.IsTrue)
	c.Assert(locationIDs[createdLocation2.GetID()], qt.IsTrue)
}

// TestLocationRegistry_Count_HappyPath tests successful location counting scenarios.
func TestLocationRegistry_Count_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty count
	count, err := registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create one location
	createTestLocation(c, registrySet.LocationRegistry)
	count, err = registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)

	// Create another location
	location2 := models.Location{
		Name:    "Second Location",
		Address: "456 Second Street",
	}
	_, err = registrySet.LocationRegistry.Create(ctx, location2)
	c.Assert(err, qt.IsNil)

	count, err = registrySet.LocationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

// TestLocationRegistry_Areas_HappyPath tests location-area relationship management.
func TestLocationRegistry_Areas_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create a test location
	location := createTestLocation(c, registrySet.LocationRegistry)

	// Initially no areas
	areas, err := registrySet.LocationRegistry.GetAreas(ctx, location.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 0)

	// Create an area
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())

	// Add area to location
	err = registrySet.LocationRegistry.AddArea(ctx, location.GetID(), area.GetID())
	c.Assert(err, qt.IsNil)

	// Verify area is added
	areas, err = registrySet.LocationRegistry.GetAreas(ctx, location.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 1)
	c.Assert(areas[0], qt.Equals, area.GetID())

	// Remove area from location
	err = registrySet.LocationRegistry.DeleteArea(ctx, location.GetID(), area.GetID())
	c.Assert(err, qt.IsNil)

	// Verify area is removed
	areas, err = registrySet.LocationRegistry.GetAreas(ctx, location.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, 0)
}

// TestLocationRegistry_Areas_UnhappyPath tests location-area relationship error scenarios.
func TestLocationRegistry_Areas_UnhappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test with non-existent location
	err := registrySet.LocationRegistry.AddArea(ctx, "non-existent-location", "some-area")
	c.Assert(err, qt.IsNotNil)

	// Test getting areas for non-existent location
	_, err = registrySet.LocationRegistry.GetAreas(ctx, "non-existent-location")
	c.Assert(err, qt.IsNotNil)

	// Test deleting area from non-existent location
	err = registrySet.LocationRegistry.DeleteArea(ctx, "non-existent-location", "some-area")
	c.Assert(err, qt.IsNotNil)
}
