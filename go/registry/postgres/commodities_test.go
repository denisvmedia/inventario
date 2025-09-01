package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestCommodityRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name      string
		commodity models.Commodity
	}{
		{
			name: "valid commodity with minimal fields",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				RegisteredDate:         models.ToPDate("2023-01-02"),
				LastModifiedDate:       models.ToPDate("2023-01-03"),
				Draft:                  false,
				TenantAwareEntityID:    models.WithTenantUserAwareEntityID("commodity1", "test-tenant-id", "test-user-id"),
			},
		},
		{
			name: "valid commodity with all fields",
			commodity: models.Commodity{
				Name:                   "Complete Commodity",
				ShortName:              "CC",
				Type:                   models.CommodityTypeWhiteGoods,
				Count:                  2,
				OriginalPrice:          decimal.NewFromFloat(250.50),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.NewFromFloat(275.00),
				CurrentPrice:           decimal.NewFromFloat(200.00),
				SerialNumber:           "SN123456",
				ExtraSerialNumbers:     []string{"SN654321", "SN789012"},
				PartNumbers:            []string{"P123", "P456"},
				Tags:                   []string{"tag1", "tag2", "tag3"},
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				RegisteredDate:         models.ToPDate("2023-01-02"),
				LastModifiedDate:       models.ToPDate("2023-01-03"),
				Comments:               "Test comments",
				Draft:                  false,
				TenantAwareEntityID:    models.WithTenantUserAwareEntityID("commodity2", "test-tenant-id", "test-user-id"),
			},
		},
		{
			name: "valid draft commodity",
			commodity: models.Commodity{
				Name:                "Draft Commodity",
				ShortName:           "DC",
				Type:                models.CommodityTypeOther,
				Count:               1,
				Status:              models.CommodityStatusInUse,
				Draft:               true,
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("commodity3", "test-tenant-id", "test-user-id"),
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

			// Setup main currency
			settingsReg, err := registrySet.SettingsRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)
			setupMainCurrency(c, settingsReg)

			commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			// Create test hierarchy
			location := createTestLocation(c, registrySet)
			area := createTestArea(c, registrySet, location.GetID())
			tc.commodity.AreaID = area.GetID()

			// Create commodity
			result, err := commodityReg.Create(ctx, tc.commodity)
			c.Assert(err, qt.IsNil)
			c.Assert(result, qt.IsNotNil)
			c.Assert(result.ID, qt.Not(qt.Equals), "")
			c.Assert(result.Name, qt.Equals, tc.commodity.Name)
			c.Assert(result.ShortName, qt.Equals, tc.commodity.ShortName)
			c.Assert(result.Type, qt.Equals, tc.commodity.Type)
			c.Assert(result.AreaID, qt.Equals, tc.commodity.AreaID)
			c.Assert(result.Count, qt.Equals, tc.commodity.Count)
			c.Assert(result.Status, qt.Equals, tc.commodity.Status)
			c.Assert(result.Draft, qt.Equals, tc.commodity.Draft)
		})
	}
}

func TestCommodityRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name      string
		commodity models.Commodity
	}{
		{
			name: "empty name",
			commodity: models.Commodity{
				Name:                   "",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				Draft:                  false,
			},
		},
		{
			name: "empty short name",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "",
				Type:                   models.CommodityTypeElectronics,
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				Draft:                  false,
			},
		},
		{
			name: "empty area ID",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				Draft:                  false,
			},
		},
		{
			name: "non-existent area",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "non-existent-area",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				Draft:                  false,
			},
		},
		{
			name: "zero count",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				Count:                  0,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				Draft:                  false,
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

			// Setup main currency
			settingsReg, err := registrySet.SettingsRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)
			setupMainCurrency(c, settingsReg)

			commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			// For valid area ID tests, create test hierarchy
			if tc.commodity.AreaID != "" && tc.commodity.AreaID != "non-existent-area" {
				location := createTestLocation(c, registrySet)
				area := createTestArea(c, registrySet, location.GetID())
				tc.commodity.AreaID = area.GetID()
			}

			// Attempt to create invalid commodity
			result, err := commodityReg.Create(ctx, tc.commodity)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestCommodityRegistry_Get_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy and commodity
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	created := createTestCommodity(c, registrySet, area.GetID())

	// Get the commodity
	result, err := commodityReg.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.Name, qt.Equals, created.Name)
	c.Assert(result.ShortName, qt.Equals, created.ShortName)
	c.Assert(result.Type, qt.Equals, created.Type)
	c.Assert(result.AreaID, qt.Equals, created.AreaID)
	c.Assert(result.Count, qt.Equals, created.Count)
	c.Assert(result.Status, qt.Equals, created.Status)
}

func TestCommodityRegistry_Get_UnhappyPath(t *testing.T) {
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

			result, err := registrySet.CommodityRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestCommodityRegistry_List_HappyPath(t *testing.T) {
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
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 0)

	// Create test hierarchy and commodities
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity1 := createTestCommodity(c, registrySet, area.GetID())
	commodity2 := createTestCommodity(c, registrySet, area.GetID())

	// List should now contain both commodities
	commodities, err = registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 2)

	// Verify the commodities are in the list
	commodityIDs := make(map[string]bool)
	for _, commodity := range commodities {
		commodityIDs[commodity.ID] = true
	}
	c.Assert(commodityIDs[commodity1.ID], qt.IsTrue)
	c.Assert(commodityIDs[commodity2.ID], qt.IsTrue)
}

func TestCommodityRegistry_Update_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Create test hierarchy and commodity
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	created := createTestCommodity(c, registrySet, area.GetID())

	// Update the commodity
	created.Name = "Updated Commodity"
	created.ShortName = "UC"
	created.Comments = "Updated comments"

	result, err := registrySet.CommodityRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.Name, qt.Equals, "Updated Commodity")
	c.Assert(result.ShortName, qt.Equals, "UC")
	c.Assert(result.Comments, qt.Equals, "Updated comments")
	c.Assert(result.AreaID, qt.Equals, created.AreaID)

	// Verify the update persisted
	retrieved, err := registrySet.CommodityRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.Name, qt.Equals, "Updated Commodity")
	c.Assert(retrieved.ShortName, qt.Equals, "UC")
	c.Assert(retrieved.Comments, qt.Equals, "Updated comments")
}

func TestCommodityRegistry_Update_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name      string
		commodity models.Commodity
	}{
		{
			name: "non-existent commodity",
			commodity: models.Commodity{
				TenantAwareEntityID:    models.WithTenantAwareEntityID("non-existent-id", "test-tenant-id"),
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "some-area-id",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				Draft:                  false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			// Setup main currency
			setupMainCurrency(c, registrySet.SettingsRegistry)

			result, err := registrySet.CommodityRegistry.Update(ctx, tc.commodity)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestCommodityRegistry_Delete_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Create test hierarchy and commodity
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	created := createTestCommodity(c, registrySet, area.GetID())

	// Delete the commodity
	err := registrySet.CommodityRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify the commodity is deleted
	result, err := registrySet.CommodityRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
}

func TestCommodityRegistry_Delete_UnhappyPath(t *testing.T) {
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

			err := registrySet.CommodityRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestCommodityRegistry_Count_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	// Initially should be 0
	count, err := registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy and commodities
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	createTestCommodity(c, registrySet, area.GetID())
	createTestCommodity(c, registrySet, area.GetID())

	// Count should now be 2
	count, err = registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

// Helper functions for creating test files

// createTestImage creates a test image for use in tests.
func createTestImage(c *qt.C, registrySet *registry.Set, commodityID string) *models.Image {
	c.Helper()

	ctx := c.Context()
	image := models.Image{
		CommodityID: commodityID,
		File: &models.File{
			Path:         "test-image",
			OriginalPath: "test-image.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
		// Note: ID will be generated server-side for security
	}

	createdImage, err := registrySet.ImageRegistry.Create(ctx, image)
	c.Assert(err, qt.IsNil)
	c.Assert(createdImage, qt.IsNotNil)

	return createdImage
}

// createTestManual creates a test manual for use in tests.
func createTestManual(c *qt.C, registrySet *registry.Set, commodityID string) *models.Manual {
	c.Helper()

	ctx := c.Context()
	manual := models.Manual{
		CommodityID: commodityID,
		File: &models.File{
			Path:         "test-manual",
			OriginalPath: "test-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
		// Note: ID will be generated server-side for security
	}

	createdManual, err := registrySet.ManualRegistry.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.IsNotNil)

	return createdManual
}

// createTestInvoice creates a test invoice for use in tests.
func createTestInvoice(c *qt.C, registrySet *registry.Set, commodityID string) *models.Invoice {
	c.Helper()

	ctx := c.Context()
	invoice := models.Invoice{
		CommodityID: commodityID,
		File: &models.File{
			Path:         "test-invoice",
			OriginalPath: "test-invoice.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-invoice-id-"+uuid.New().String(), "test-tenant-id", "test-user-id"),
	}

	createdInvoice, err := registrySet.InvoiceRegistry.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.IsNotNil)

	return createdInvoice
}

// Image-related tests

func TestCommodityRegistry_GetImages_WithCreatedImage_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Create test image (automatically linked via commodity_id)
	image := createTestImage(c, registrySet, commodity.ID)

	// Verify the image is automatically linked
	images, err := commodityReg.GetImages(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(images), qt.Equals, 1)
	c.Assert(images[0], qt.Equals, image.ID)
}

func TestCommodityRegistry_GetImages_WithInvalidCommodity_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name        string
		commodityID string
	}{
		{
			name:        "non-existent commodity",
			commodityID: "non-existent-commodity",
		},
		{
			name:        "empty commodity ID",
			commodityID: "",
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

			commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			images, err := commodityReg.GetImages(ctx, tc.commodityID)
			c.Assert(err, qt.IsNotNil)
			c.Assert(images, qt.IsNil)
		})
	}
}

func TestCommodityRegistry_GetImages_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Initially should have no images
	images, err := commodityReg.GetImages(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(images), qt.Equals, 0)

	// Create images (automatically linked via commodity_id)
	image1 := createTestImage(c, registrySet, commodity.ID)
	image2 := createTestImage(c, registrySet, commodity.ID)

	// Should now have 2 images
	images, err = commodityReg.GetImages(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(images), qt.Equals, 2)

	// Verify the image IDs are correct
	imageIDs := make(map[string]bool)
	for _, imageID := range images {
		imageIDs[imageID] = true
	}
	c.Assert(imageIDs[image1.ID], qt.IsTrue)
	c.Assert(imageIDs[image2.ID], qt.IsTrue)
}

func TestCommodityRegistry_GetImages_EmptyCommodity_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Should have no images initially
	images, err := commodityReg.GetImages(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(images), qt.Equals, 0)
}

// Manual-related tests

func TestCommodityRegistry_GetManuals_WithCreatedManual_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Create test manual (automatically linked via commodity_id)
	manual := createTestManual(c, registrySet, commodity.ID)

	// Verify the manual is automatically linked
	manuals, err := commodityReg.GetManuals(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(manuals), qt.Equals, 1)
	c.Assert(manuals[0], qt.Equals, manual.ID)
}

func TestCommodityRegistry_GetManuals_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Initially should have no manuals
	manuals, err := commodityReg.GetManuals(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(manuals), qt.Equals, 0)

	// Create manuals (automatically linked via commodity_id)
	manual1 := createTestManual(c, registrySet, commodity.ID)
	manual2 := createTestManual(c, registrySet, commodity.ID)

	// Should now have 2 manuals
	manuals, err = commodityReg.GetManuals(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(manuals), qt.Equals, 2)

	// Verify the manual IDs are correct
	manualIDs := make(map[string]bool)
	for _, manualID := range manuals {
		manualIDs[manualID] = true
	}
	c.Assert(manualIDs[manual1.ID], qt.IsTrue)
	c.Assert(manualIDs[manual2.ID], qt.IsTrue)
}

func TestCommodityRegistry_GetManuals_EmptyCommodity_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Should have no manuals initially
	manuals, err := commodityReg.GetManuals(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(manuals), qt.Equals, 0)
}

// Invoice-related tests

func TestCommodityRegistry_GetInvoices_WithCreatedInvoice_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Create test invoice (automatically linked via commodity_id)
	invoice := createTestInvoice(c, registrySet, commodity.ID)

	// Verify the invoice is automatically linked
	invoices, err := commodityReg.GetInvoices(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invoices), qt.Equals, 1)
	c.Assert(invoices[0], qt.Equals, invoice.ID)
}

func TestCommodityRegistry_GetInvoices_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Initially should have no invoices
	invoices, err := commodityReg.GetInvoices(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invoices), qt.Equals, 0)

	// Create invoices (automatically linked via commodity_id)
	invoice1 := createTestInvoice(c, registrySet, commodity.ID)
	invoice2 := createTestInvoice(c, registrySet, commodity.ID)

	// Should now have 2 invoices
	invoices, err = commodityReg.GetInvoices(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invoices), qt.Equals, 2)

	// Verify the invoice IDs are correct
	invoiceIDs := make(map[string]bool)
	for _, invoiceID := range invoices {
		invoiceIDs[invoiceID] = true
	}
	c.Assert(invoiceIDs[invoice1.ID], qt.IsTrue)
	c.Assert(invoiceIDs[invoice2.ID], qt.IsTrue)
}

func TestCommodityRegistry_GetInvoices_EmptyCommodity_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	commodityReg, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create test hierarchy
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())

	// Should have no invoices initially
	invoices, err := commodityReg.GetInvoices(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invoices), qt.Equals, 0)
}
