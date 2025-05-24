package postgresql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// TestManualRegistry_Create_HappyPath tests successful manual creation scenarios.
func TestManualRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name   string
		manual models.Manual
	}{
		{
			name: "basic manual",
			manual: models.Manual{
				File: &models.File{
					Path:         "test-manual",
					OriginalPath: "test-manual.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
		{
			name: "manual with special characters",
			manual: models.Manual{
				File: &models.File{
					Path:         "manual-café",
					OriginalPath: "manual café.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Create test hierarchy
			location := createTestLocation(c, registrySet.LocationRegistry)
			area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
			commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
			tc.manual.CommodityID = commodity.GetID()

			// Create manual
			createdManual, err := registrySet.ManualRegistry.Create(ctx, tc.manual)
			c.Assert(err, qt.IsNil)
			c.Assert(createdManual, qt.IsNotNil)
			c.Assert(createdManual.GetID(), qt.Not(qt.Equals), "")
			c.Assert(createdManual.CommodityID, qt.Equals, tc.manual.CommodityID)
			c.Assert(createdManual.File.Path, qt.Equals, tc.manual.File.Path)
			c.Assert(createdManual.File.OriginalPath, qt.Equals, tc.manual.File.OriginalPath)
			c.Assert(createdManual.File.Ext, qt.Equals, tc.manual.File.Ext)
			c.Assert(createdManual.File.MIMEType, qt.Equals, tc.manual.File.MIMEType)

			// Verify count
			count, err := registrySet.ManualRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 1)
		})
	}
}

// TestManualRegistry_Create_UnhappyPath tests manual creation error scenarios.
func TestManualRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name   string
		manual models.Manual
	}{
		{
			name: "empty commodity ID",
			manual: models.Manual{
				CommodityID: "",
				File: &models.File{
					Path:         "test-manual",
					OriginalPath: "test-manual.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
		{
			name: "non-existent commodity ID",
			manual: models.Manual{
				CommodityID: "non-existent-commodity",
				File: &models.File{
					Path:         "test-manual",
					OriginalPath: "test-manual.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
		{
			name: "nil file",
			manual: models.Manual{
				CommodityID: "some-commodity-id",
				File:        nil,
			},
		},
		{
			name: "empty path",
			manual: models.Manual{
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "",
					OriginalPath: "test-manual.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid commodity ID tests, create test hierarchy
			if tc.manual.CommodityID != "" && tc.manual.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
				commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
				tc.manual.CommodityID = commodity.GetID()
			}

			// Attempt to create invalid manual
			_, err := registrySet.ManualRegistry.Create(ctx, tc.manual)
			c.Assert(err, qt.IsNotNil)

			// Verify count remains zero
			count, err := registrySet.ManualRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 0)
		})
	}
}

// TestManualRegistry_Get_HappyPath tests successful manual retrieval scenarios.
func TestManualRegistry_Get_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	manual := createTestManual(c, registrySet.ManualRegistry, commodity.GetID())

	// Get the manual
	retrievedManual, err := registrySet.ManualRegistry.Get(ctx, manual.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedManual, qt.IsNotNil)
	c.Assert(retrievedManual.GetID(), qt.Equals, manual.GetID())
	c.Assert(retrievedManual.CommodityID, qt.Equals, manual.CommodityID)
	c.Assert(retrievedManual.File.Path, qt.Equals, manual.File.Path)
	c.Assert(retrievedManual.File.OriginalPath, qt.Equals, manual.File.OriginalPath)
	c.Assert(retrievedManual.File.Ext, qt.Equals, manual.File.Ext)
	c.Assert(retrievedManual.File.MIMEType, qt.Equals, manual.File.MIMEType)
}

// TestManualRegistry_Get_UnhappyPath tests manual retrieval error scenarios.
func TestManualRegistry_Get_UnhappyPath(t *testing.T) {
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

			// Try to get non-existent manual
			_, err := registrySet.ManualRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorMatches, ".*not found.*")
		})
	}
}

// TestManualRegistry_Update_HappyPath tests successful manual update scenarios.
func TestManualRegistry_Update_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	manual := createTestManual(c, registrySet.ManualRegistry, commodity.GetID())

	// Update the manual
	manual.File.Path = "updated-manual"

	updatedManual, err := registrySet.ManualRegistry.Update(ctx, *manual)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedManual, qt.IsNotNil)
	c.Assert(updatedManual.GetID(), qt.Equals, manual.GetID())
	c.Assert(updatedManual.File.Path, qt.Equals, "updated-manual")

	// Verify the update persisted
	retrievedManual, err := registrySet.ManualRegistry.Get(ctx, manual.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedManual.File.Path, qt.Equals, "updated-manual")
}

// TestManualRegistry_Update_UnhappyPath tests manual update error scenarios.
func TestManualRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name   string
		manual models.Manual
	}{
		{
			name: "non-existent manual",
			manual: models.Manual{
				EntityID:    models.EntityID{ID: "non-existent-id"},
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "test-manual",
					OriginalPath: "test-manual.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
		{
			name: "empty path",
			manual: models.Manual{
				EntityID:    models.EntityID{ID: "some-id"},
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "",
					OriginalPath: "test-manual.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
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
			_, err := registrySet.ManualRegistry.Update(ctx, tc.manual)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestManualRegistry_Delete_HappyPath tests successful manual deletion scenarios.
func TestManualRegistry_Delete_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	manual := createTestManual(c, registrySet.ManualRegistry, commodity.GetID())

	// Verify manual exists
	_, err := registrySet.ManualRegistry.Get(ctx, manual.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the manual
	err = registrySet.ManualRegistry.Delete(ctx, manual.GetID())
	c.Assert(err, qt.IsNil)

	// Verify manual is deleted
	_, err = registrySet.ManualRegistry.Get(ctx, manual.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

// TestManualRegistry_Delete_UnhappyPath tests manual deletion error scenarios.
func TestManualRegistry_Delete_UnhappyPath(t *testing.T) {
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

			// Try to delete non-existent manual
			err := registrySet.ManualRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestManualRegistry_List_HappyPath tests successful manual listing scenarios.
func TestManualRegistry_List_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty list
	manuals, err := registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.HasLen, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	manual1 := createTestManual(c, registrySet.ManualRegistry, commodity.GetID())

	manual2 := models.Manual{
		CommodityID: commodity.GetID(),
		File: &models.File{
			Path:         "second-manual",
			OriginalPath: "second-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	createdManual2, err := registrySet.ManualRegistry.Create(ctx, manual2)
	c.Assert(err, qt.IsNil)

	// List all manuals
	manuals, err = registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.HasLen, 2)

	// Verify manuals are in the list
	manualIDs := make(map[string]bool)
	for _, manual := range manuals {
		manualIDs[manual.GetID()] = true
	}
	c.Assert(manualIDs[manual1.GetID()], qt.IsTrue)
	c.Assert(manualIDs[createdManual2.GetID()], qt.IsTrue)
}

// TestManualRegistry_Count_HappyPath tests successful manual counting scenarios.
func TestManualRegistry_Count_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty count
	count, err := registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	createTestManual(c, registrySet.ManualRegistry, commodity.GetID())

	count, err = registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)

	// Create another manual
	manual2 := models.Manual{
		CommodityID: commodity.GetID(),
		File: &models.File{
			Path:         "second-manual",
			OriginalPath: "second-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	_, err = registrySet.ManualRegistry.Create(ctx, manual2)
	c.Assert(err, qt.IsNil)

	count, err = registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

// TestManualRegistry_CascadeDelete tests that deleting a commodity cascades to manuals.
func TestManualRegistry_CascadeDelete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())
	manual := createTestManual(c, registrySet.ManualRegistry, commodity.GetID())

	// Verify manual exists
	_, err := registrySet.ManualRegistry.Get(ctx, manual.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the commodity (should cascade to manual)
	err = registrySet.CommodityRegistry.Delete(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)

	// Verify manual is also deleted due to cascade
	_, err = registrySet.ManualRegistry.Get(ctx, manual.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}