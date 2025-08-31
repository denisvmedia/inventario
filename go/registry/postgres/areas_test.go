package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
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
				// Note: ID will be generated server-side for security
			},
		},
		{
			name: "valid area with minimal fields",
			area: models.Area{
				Name: "Secondary Storage",
				// Note: ID will be generated server-side for security
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			registrySet, cleanup := setupTestRegistrySet(t)
			c.Cleanup(cleanup)

			areaReg, err := registrySet.AreaRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			// Create a test location first
			location := createTestLocation(c, registrySet)
			tc.area.LocationID = location.GetID()

			// Create area
			result, err := areaReg.Create(ctx, tc.area)
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
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()


			// For the non-existent location test, we don't need to create a location
			// For other tests, create a location if LocationID is not empty
			if tc.area.LocationID != "" && tc.area.LocationID != "non-existent-location" {

				location := createTestLocation(c, registrySet)
				tc.area.LocationID = location.GetID()
			}

			result, err := areaReg.Create(ctx, tc.area)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestAreaRegistry_Get_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})



	// Create a test location and area
	location := createTestLocation(c, registrySet)
	created := createTestArea(c, registrySet, location.ID)

	// Get the area
	result, err := areaReg.Get(ctx, created.ID)
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
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})


			result, err := areaReg.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestAreaRegistry_List_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})



	// Initially should be empty
	areas, err := areaReg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 0)

	// Create test location and areas
	location := createTestLocation(c, registrySet)
	area1 := createTestArea(c, registrySet, location.ID)
	area2 := createTestArea(c, registrySet, location.ID)

	// List should now contain both areas
	areas, err = areaReg.List(ctx)
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
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})



	// Create test location and area
	location := createTestLocation(c, registrySet)
	created := createTestArea(c, registrySet, location.ID)

	// Update the area
	created.Name = "Updated Area"

	result, err := areaReg.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.Name, qt.Equals, "Updated Area")
	c.Assert(result.LocationID, qt.Equals, created.LocationID)

	// Verify the update persisted
	retrieved, err := areaReg.Get(ctx, created.ID)
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
				TenantAwareEntityID: models.WithTenantAwareEntityID("non-existent-id", "test-tenant-id"),
				Name:                "Test Area",
				LocationID:          "some-location-id",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})


			result, err := areaReg.Update(ctx, tc.area)
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
	ctx = appctx.WithUser(ctx, &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})



	// Create test location and area
	location := createTestLocation(c, registrySet)
	created := createTestArea(c, registrySet, location.ID)

	// Delete the area
	err = areaReg.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify the area is deleted
	result, err := areaReg.Get(ctx, created.ID)
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
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})


			err = areaReg.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestAreaRegistry_Delete_WithCommodities_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})



	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)

	// Commodity is automatically linked to area via area_id field
	// Try to delete the area - should fail because it has commodities
	err = areaReg.Delete(ctx, area.ID)
	c.Assert(err, qt.IsNotNil)

	// Verify the area still exists
	result, err := areaReg.Get(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)

	// Verify the commodity still exists
	_ = commodity // commodity is created but we don't need to verify it here
}

func TestAreaRegistry_Count_HappyPath(t *testing.T) {
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



	// Initially should be 0
	count, err := areaReg.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test location and areas
	location := createTestLocation(c, registrySet)
	createTestArea(c, registrySet, location.ID)
	createTestArea(c, registrySet, location.ID)

	// Count should now be 2
	count, err = areaReg.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

func TestAreaRegistry_GetCommodities_WithCreatedCommodity_HappyPath(t *testing.T) {
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



	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)

	// Commodity is automatically linked to area via area_id field
	// Verify the commodity is linked
	commodities, err := areaReg.GetCommodities(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)
	c.Assert(commodities[0], qt.Equals, commodity.ID)
}

func TestAreaRegistry_GetCommodities_WithInvalidArea_UnhappyPath(t *testing.T) {
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
			ctx := appctx.WithUser(c.Context(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-user-id"},
					TenantID: "test-tenant-id",
				},
			})


			commodities, err := areaReg.GetCommodities(ctx, tc.areaID)
			c.Assert(err, qt.IsNotNil)
			c.Assert(commodities, qt.IsNil)
		})
	}
}

func TestAreaRegistry_GetCommodities_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})



	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)

	// Initially should have no commodities
	commodities, err := areaReg.GetCommodities(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 0)

	// Create commodities (they are automatically linked via area_id)
	commodity1 := createTestCommodity(c, registrySet, area.ID)
	commodity2 := createTestCommodity(c, registrySet, area.ID)

	// Should now have 2 commodities
	commodities, err = areaReg.GetCommodities(ctx, area.ID)
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

func TestAreaRegistry_GetCommodities_EmptyArea_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})



	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.ID)

	// Should have no commodities initially
	commodities, err := areaReg.GetCommodities(ctx, area.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 0)
}
