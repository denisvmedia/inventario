package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

func TestCommodityRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name      string
		commodity models.Commodity
	}{
		{
			name: "valid commodity with minimal fields",
			commodity: models.Commodity{
				Name:                     "Test Commodity",
				ShortName:                "TC",
				Type:                     models.CommodityTypeElectronics,
				Count:                    1,
				OriginalPrice:            decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:    "USD",
				ConvertedOriginalPrice:   decimal.Zero,
				CurrentPrice:             decimal.NewFromFloat(90.00),
				Status:                   models.CommodityStatusInUse,
				PurchaseDate:             models.ToPDate("2023-01-01"),
				RegisteredDate:           models.ToPDate("2023-01-02"),
				LastModifiedDate:         models.ToPDate("2023-01-03"),
				Draft:                    false,
				TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("commodity1", "test-tenant-id", "", "test-user-id"),
			},
		},
		{
			name: "valid commodity with all fields",
			commodity: models.Commodity{
				Name:                     "Complete Commodity",
				ShortName:                "CC",
				Type:                     models.CommodityTypeWhiteGoods,
				Count:                    2,
				OriginalPrice:            decimal.NewFromFloat(250.50),
				OriginalPriceCurrency:    "EUR",
				ConvertedOriginalPrice:   decimal.NewFromFloat(275.00),
				CurrentPrice:             decimal.NewFromFloat(200.00),
				SerialNumber:             "SN123456",
				ExtraSerialNumbers:       []string{"SN654321", "SN789012"},
				PartNumbers:              []string{"P123", "P456"},
				Tags:                     []string{"tag1", "tag2", "tag3"},
				Status:                   models.CommodityStatusInUse,
				PurchaseDate:             models.ToPDate("2023-01-01"),
				RegisteredDate:           models.ToPDate("2023-01-02"),
				LastModifiedDate:         models.ToPDate("2023-01-03"),
				Comments:                 "Test comments",
				Draft:                    false,
				TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("commodity2", "test-tenant-id", "", "test-user-id"),
			},
		},
		{
			name: "valid draft commodity",
			commodity: models.Commodity{
				Name:                     "Draft Commodity",
				ShortName:                "DC",
				Type:                     models.CommodityTypeOther,
				Count:                    1,
				Status:                   models.CommodityStatusInUse,
				Draft:                    true,
				TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("commodity3", "test-tenant-id", "", "test-user-id"),
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

			// Setup main currency (registries are already user-aware from setupTestRegistrySet)
			setupMainCurrency(c, registrySet.SettingsRegistry)

			// Create test hierarchy
			location := createTestLocation(c, registrySet)
			area := createTestArea(c, registrySet, location.GetID())
			tc.commodity.AreaID = area.GetID()

			// Create commodity
			result, err := registrySet.CommodityRegistry.Create(ctx, tc.commodity)
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

			// Setup main currency (registries are already user-aware from setupTestRegistrySet)
			setupMainCurrency(c, registrySet.SettingsRegistry)

			// For valid area ID tests, create test hierarchy
			if tc.commodity.AreaID != "" && tc.commodity.AreaID != "non-existent-area" {
				location := createTestLocation(c, registrySet)
				area := createTestArea(c, registrySet, location.GetID())
				tc.commodity.AreaID = area.GetID()
			}

			// Attempt to create invalid commodity
			result, err := registrySet.CommodityRegistry.Create(ctx, tc.commodity)
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

	// Registry is already user-aware from setupTestRegistrySet

	// Create test hierarchy and commodity
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	created := createTestCommodity(c, registrySet, area.GetID())

	// Get the commodity (registry is already user-aware)
	result, err := registrySet.CommodityRegistry.Get(ctx, created.ID)
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
	c.Assert(commodities, qt.HasLen, 0)

	// Create test hierarchy and commodities
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())
	commodity1 := createTestCommodity(c, registrySet, area.GetID())
	commodity2 := createTestCommodity(c, registrySet, area.GetID())

	// List should now contain both commodities
	commodities, err = registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 2)

	// Verify the commodities are in the list
	commodityIDs := make(map[string]bool)
	for _, commodity := range commodities {
		commodityIDs[commodity.ID] = true
	}
	c.Assert(commodityIDs[commodity1.ID], qt.IsTrue)
	c.Assert(commodityIDs[commodity2.ID], qt.IsTrue)
}

func TestCommodityRegistry_List_SortedByPurchaseDate(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
	})

	setupMainCurrency(c, registrySet.SettingsRegistry)
	location := createTestLocation(c, registrySet)
	area := createTestArea(c, registrySet, location.GetID())

	// Create commodities with different purchase dates (out of order)
	makeCommodity := func(name, purchaseDate string) models.Commodity {
		c := models.Commodity{
			TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", "test-tenant-id", "", "test-user-id"),
			Name:                     name,
			ShortName:                name,
			Type:                     models.CommodityTypeElectronics,
			AreaID:                   area.GetID(),
			Count:                    1,
			OriginalPrice:            decimal.NewFromFloat(10),
			OriginalPriceCurrency:    "USD",
			ConvertedOriginalPrice:   decimal.Zero,
			CurrentPrice:             decimal.NewFromFloat(10),
			Status:                   models.CommodityStatusInUse,
			Draft:                    false,
		}
		if purchaseDate != "" {
			c.PurchaseDate = models.ToPDate(models.Date(purchaseDate))
		}
		return c
	}

	_, err := registrySet.CommodityRegistry.Create(ctx, makeCommodity("older", "2021-06-15"))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.CommodityRegistry.Create(ctx, makeCommodity("newest", "2023-12-01"))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.CommodityRegistry.Create(ctx, makeCommodity("middle", "2022-03-20"))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.CommodityRegistry.Create(ctx, makeCommodity("no_date", ""))
	c.Assert(err, qt.IsNil)

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 4)

	// Expect descending order: newest → middle → older → no_date (nil last)
	c.Assert(commodities[0].Name, qt.Equals, "newest")
	c.Assert(commodities[1].Name, qt.Equals, "middle")
	c.Assert(commodities[2].Name, qt.Equals, "older")
	c.Assert(commodities[3].Name, qt.Equals, "no_date")
	c.Assert(commodities[3].PurchaseDate, qt.IsNil)
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
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{EntityID: models.EntityID{ID: "non-existent-id"}, TenantID: "test-tenant-id"},
				Name:                     "Test Commodity",
				ShortName:                "TC",
				Type:                     models.CommodityTypeElectronics,
				AreaID:                   "some-area-id",
				Count:                    1,
				OriginalPrice:            decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:    "USD",
				ConvertedOriginalPrice:   decimal.Zero,
				CurrentPrice:             decimal.NewFromFloat(90.00),
				Status:                   models.CommodityStatusInUse,
				PurchaseDate:             models.ToPDate("2023-01-01"),
				Draft:                    false,
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
