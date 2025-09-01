package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
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
				// Note: ID will be generated server-side for security
			},
		},
		{
			name: "valid location with minimal fields",
			location: models.Location{
				Name:    "Warehouse",
				Address: "456 Storage Ave",
				// Note: ID will be generated server-side for security
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()
			// Get the first seeded user to use as the current user
			users, err := registrySet.UserRegistry.List(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(len(users), qt.Not(qt.Equals), 0, qt.Commentf("No users found - ensure setupTestTenantAndUser was called"))

			// Use the first seeded user (should be the admin user created by seeddata)
			seededUser := users[0]
			ctx = appctx.WithUser(ctx, seededUser)

			locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			result, err := locationReg.Create(ctx, tc.location)
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
			ctx = appctx.WithUser(ctx, &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			result, err := locationReg.Create(ctx, tc.location)
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
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create a test location
	created := createTestLocation(c, registrySet)

	// Get the location
	result, err := locationReg.Get(ctx, created.ID)
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
			ctx = appctx.WithUser(ctx, &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			result, err := locationReg.Get(ctx, tc.id)
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
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Initially should be empty
	locations, err := locationReg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations), qt.Equals, 0)

	// Create test locations
	location1 := createTestLocation(c, registrySet)
	location2 := createTestLocation(c, registrySet)

	// List should now contain both locations
	locations, err = locationReg.List(ctx)
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
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create a test location
	created := createTestLocation(c, registrySet)

	// Update the location
	created.Name = "Updated Location"
	created.Address = "Updated Address"

	result, err := locationReg.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.Name, qt.Equals, "Updated Location")
	c.Assert(result.Address, qt.Equals, "Updated Address")

	// Verify the update persisted
	retrieved, err := locationReg.Get(ctx, created.ID)
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
				TenantAwareEntityID: models.WithTenantAwareEntityID("non-existent-id", "test-tenant-id"),
				Name:                "Test Location",
				Address:             "Test Address",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()
			ctx = appctx.WithUser(ctx, &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			result, err := locationReg.Update(ctx, tc.location)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestLocationRegistry_Delete_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create a test location
	created := createTestLocation(c, registrySet)

	// Delete the location
	err = locationReg.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify the location is deleted
	result, err := locationReg.Get(ctx, created.ID)
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
			ctx := c.Context()
			ctx = appctx.WithUser(ctx, &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			err = locationReg.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestLocationRegistry_Delete_WithAreas_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	areaReg, err := registrySet.AreaRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create a test location
	location := createTestLocation(c, registrySet)

	// Create an area in the location (area is automatically linked via location_id)
	area := createTestArea(c, registrySet, location.ID)

	// Try to delete the location - should fail because it has areas
	err = locationReg.Delete(ctx, location.ID)
	c.Assert(err, qt.IsNotNil)

	// Verify the location still exists
	result, err := locationReg.Get(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)

	// Verify the area still exists
	areaResult, err := areaReg.Get(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(areaResult, qt.IsNotNil)
}

func TestLocationRegistry_Count_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	locationReg := registrySet.LocationRegistry.MustWithCurrentUser(ctx)

	// Initially should be 0
	count, err := locationReg.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test locations
	createTestLocation(c, registrySet)
	createTestLocation(c, registrySet)

	// Count should now be 2
	count, err = locationReg.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

func TestLocationRegistry_GetAreas_WithCreatedArea_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Create a test location and area
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)

	locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Verify the area is automatically linked to the location
	areas, err := locationReg.GetAreas(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 1)
	c.Assert(areas[0], qt.Equals, area.ID)
}

func TestLocationRegistry_GetAreas_WithInvalidLocation_UnhappyPath(t *testing.T) {
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
			ctx := c.Context()
			ctx = appctx.WithUser(ctx, &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			areas, err := locationReg.GetAreas(ctx, tc.locationID)
			c.Assert(err, qt.IsNotNil)
			c.Assert(areas, qt.IsNil)
		})
	}
}

func TestLocationRegistry_GetAreas_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Create a test location
	location := createTestLocation(c, registrySet)

	locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Initially should have no areas
	areas, err := locationReg.GetAreas(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 0)

	// Create areas (they are automatically linked via location_id)
	area1 := createTestArea(c, registrySet, location.ID)
	area2 := createTestArea(c, registrySet, location.ID)

	// Should now have 2 areas
	areas, err = locationReg.GetAreas(ctx, location.ID)
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

func TestLocationRegistry_GetAreas_EmptyLocation_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Create a test location
	location := createTestLocation(c, registrySet)

	locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Should have no areas initially
	areas, err := locationReg.GetAreas(ctx, location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 0)
}
