package commonsql_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestManualRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name   string
		manual models.Manual
	}{
		{
			name: "valid manual with all fields",
			manual: models.Manual{
				File: &models.File{
					Path:         "test-manual",
					OriginalPath: "test-manual.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-manual-id", "default-tenant", "test-user-id"),
			},
		},
		{
			name: "valid manual with different format",
			manual: models.Manual{
				File: &models.File{
					Path:         "another-manual",
					OriginalPath: "another-manual.docx",
					Ext:          ".docx",
					MIMEType:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				},
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-manual-id2", "default-tenant", "test-user-id"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			registrySet, cleanup := setupTestRegistrySet(t)
			c.Cleanup(cleanup)

			// Create test hierarchy
			location := createTestLocation(c, registrySet.LocationRegistry)
			area := createTestArea(c, registrySet.AreaRegistry, location.ID)
			commodity := createTestCommodity(c, registrySet, area.ID)

			// Set commodity ID
			tc.manual.CommodityID = commodity.ID

			// Create manual
			result, err := registrySet.ManualRegistry.Create(ctx, tc.manual)
			c.Assert(err, qt.IsNil)
			c.Assert(result, qt.IsNotNil)
			c.Assert(result.ID, qt.Not(qt.Equals), "")
			c.Assert(result.CommodityID, qt.Equals, tc.manual.CommodityID)
			c.Assert(result.File.Path, qt.Equals, tc.manual.File.Path)
			c.Assert(result.File.OriginalPath, qt.Equals, tc.manual.File.OriginalPath)
			c.Assert(result.File.Ext, qt.Equals, tc.manual.File.Ext)
			c.Assert(result.File.MIMEType, qt.Equals, tc.manual.File.MIMEType)
		})
	}
}

func TestManualRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name   string
		manual models.Manual
	}{
		{
			name: "missing commodity ID",
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
			name: "non-existent commodity",
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
			name: "missing file",
			manual: models.Manual{
				CommodityID: "some-commodity-id",
			},
		},
		{
			name:   "empty manual",
			manual: models.Manual{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid commodity ID tests, create test hierarchy
			if tc.manual.CommodityID != "" && tc.manual.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				area := createTestArea(c, registrySet.AreaRegistry, location.ID)
				commodity := createTestCommodity(c, registrySet, area.ID)
				tc.manual.CommodityID = commodity.ID
			}

			// Attempt to create invalid manual
			result, err := registrySet.ManualRegistry.Create(ctx, tc.manual)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestManualRegistry_Get_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy and manual
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestManual(c, registrySet, commodity.ID)

	// Get the manual
	result, err := registrySet.ManualRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.CommodityID, qt.Equals, created.CommodityID)
	c.Assert(result.File.Path, qt.Equals, created.File.Path)
	c.Assert(result.File.OriginalPath, qt.Equals, created.File.OriginalPath)
	c.Assert(result.File.Ext, qt.Equals, created.File.Ext)
	c.Assert(result.File.MIMEType, qt.Equals, created.File.MIMEType)
}

func TestManualRegistry_Get_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent manual",
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

			result, err := registrySet.ManualRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestManualRegistry_List_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Initially should be empty
	manuals, err := registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(manuals), qt.Equals, 0)

	// Create test hierarchy and manuals
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	manual1 := createTestManual(c, registrySet, commodity.ID)
	manual2 := createTestManual(c, registrySet, commodity.ID)

	// List should now contain both manuals
	manuals, err = registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(manuals), qt.Equals, 2)

	// Verify the manuals are correct
	manualIDs := make(map[string]bool)
	for _, manual := range manuals {
		manualIDs[manual.ID] = true
	}
	c.Assert(manualIDs[manual1.ID], qt.IsTrue)
	c.Assert(manualIDs[manual2.ID], qt.IsTrue)
}

func TestManualRegistry_Update_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy and manual
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestManual(c, registrySet, commodity.ID)

	// Update the manual
	created.File.Path = "updated-manual-path"
	created.File.MIMEType = "text/plain"

	result, err := registrySet.ManualRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.File.Path, qt.Equals, "updated-manual-path")
	c.Assert(result.File.MIMEType, qt.Equals, "text/plain")

	// Verify the update persisted
	retrieved, err := registrySet.ManualRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.File.Path, qt.Equals, "updated-manual-path")
	c.Assert(retrieved.File.MIMEType, qt.Equals, "text/plain")
}

func TestManualRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name   string
		manual models.Manual
	}{
		{
			name: "non-existent manual",
			manual: models.Manual{
				TenantAwareEntityID: models.WithTenantAwareEntityID("non-existent-id", "default-tenant"),
				CommodityID:         "some-commodity-id",
				File: &models.File{
					Path:         "test-manual",
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
			ctx := c.Context()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid commodity ID tests, create test hierarchy
			if tc.manual.CommodityID != "" && tc.manual.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				area := createTestArea(c, registrySet.AreaRegistry, location.ID)
				commodity := createTestCommodity(c, registrySet, area.ID)
				tc.manual.CommodityID = commodity.ID
			}

			// Attempt to update non-existent manual
			result, err := registrySet.ManualRegistry.Update(ctx, tc.manual)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestManualRegistry_Delete_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy and manual
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestManual(c, registrySet, commodity.ID)

	// Delete the manual
	err := registrySet.ManualRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify the manual is deleted
	result, err := registrySet.ManualRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
}

func TestManualRegistry_Delete_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent manual",
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

			err := registrySet.ManualRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestManualRegistry_Count_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Initially should be 0
	count, err := registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy and manuals
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	createTestManual(c, registrySet, commodity.ID)
	createTestManual(c, registrySet, commodity.ID)

	// Count should now be 2
	count, err = registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}
