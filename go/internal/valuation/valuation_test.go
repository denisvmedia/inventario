package valuation_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/valuation"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// setupTestRegistry creates a test registry with test data
func setupTestRegistry(c *qt.C, mainCurrency string) *registry.Set {
	c.Helper()

	nonMainCurrency := "USD"

	if mainCurrency == "USD" {
		nonMainCurrency = "EUR"
	}

	// Create a memory factory set for testing
	factorySet := memory.NewFactorySet()

	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})

	// Create user-aware registry set
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Set main currency
	err := registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	// Create locations
	location1, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Location 1",
	})
	c.Assert(err, qt.IsNil)

	location2, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Location 2",
	})
	c.Assert(err, qt.IsNil)

	// Create areas
	areaRegistry := registrySet.AreaRegistry
	area1, err := areaRegistry.Create(ctx, models.Area{
		Name:       "Area 1",
		LocationID: location1.ID,
	})
	c.Assert(err, qt.IsNil)

	area2, err := areaRegistry.Create(ctx, models.Area{
		Name:       "Area 2",
		LocationID: location2.ID,
	})
	c.Assert(err, qt.IsNil)

	area3, err := registrySet.AreaRegistry.Create(ctx, models.Area{
		Name:       "Area 3",
		LocationID: location2.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create commodities
	commodityRegistry := registrySet.CommodityRegistry
	_, err = commodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Commodity 1",
		ShortName:             "C1",
		AreaID:                area1.ID,
		Count:                 2,
		OriginalPrice:         decimal.NewFromFloat(100.00),
		OriginalPriceCurrency: models.Currency(mainCurrency),
		CurrentPrice:          decimal.NewFromFloat(100.00), // 100
		Status:                models.CommodityStatusInUse,
		Draft:                 false,
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = commodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Commodity 2",
		ShortName:             "C2",
		AreaID:                area1.ID,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(200.00), // 0: invalid
		OriginalPriceCurrency: models.Currency(nonMainCurrency),
		// no converted price and no current price, so it should not be counted
		Status:       models.CommodityStatusInUse,
		Draft:        false,
		Type:         models.CommodityTypeElectronics,
		PurchaseDate: models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = commodityRegistry.Create(ctx, models.Commodity{
		Name:                   "Commodity 3",
		ShortName:              "C3",
		AreaID:                 area2.ID,
		Count:                  3,
		OriginalPrice:          decimal.NewFromFloat(300.00), // 300
		OriginalPriceCurrency:  models.Currency(mainCurrency),
		ConvertedOriginalPrice: decimal.NewFromFloat(0),
		Status:                 models.CommodityStatusInUse,
		Draft:                  false,
		Type:                   models.CommodityTypeElectronics,
		PurchaseDate:           models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = commodityRegistry.Create(ctx, models.Commodity{ // 400
		Name:                  "Commodity 4",
		ShortName:             "C4",
		AreaID:                area2.ID,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(400.00),
		OriginalPriceCurrency: models.Currency(mainCurrency),
		Status:                models.CommodityStatusInUse,
		Draft:                 false,
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = commodityRegistry.Create(ctx, models.Commodity{ // 0: sold
		Name:                  "Commodity 5",
		ShortName:             "C5",
		AreaID:                area3.ID,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(500.00),
		OriginalPriceCurrency: models.Currency(nonMainCurrency),
		CurrentPrice:          decimal.NewFromFloat(500.00),
		Status:                models.CommodityStatusSold, // Not in use
		Draft:                 false,
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = registrySet.CommodityRegistry.Create(ctx, models.Commodity{ // 0: draft
		Name:                  "Commodity 6",
		ShortName:             "C6",
		AreaID:                area3.ID,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(600.00),
		OriginalPriceCurrency: models.Currency(nonMainCurrency),
		CurrentPrice:          decimal.NewFromFloat(600.00),
		Status:                models.CommodityStatusInUse,
		Draft:                 true, // Value
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	return registrySet
}

func TestValuator_CalculateGlobalTotalValue(t *testing.T) {
	c := qt.New(t)

	// Test with USD as main currency
	c.Run("USD as main currency", func(c *qt.C) {
		// Setup test registry with USD as main currency
		registrySet := setupTestRegistry(c, "USD")
		valuator := valuation.NewValuator(registrySet)

		// Calculate global total value
		total, err := valuator.CalculateGlobalTotalValue()
		c.Assert(err, qt.IsNil)

		// Expected total: 100 + 300 + 400 = 800 (prices already represent total value for all items)
		expectedTotal := decimal.NewFromFloat(800.00)
		c.Assert(total.Equal(expectedTotal), qt.IsTrue, qt.Commentf("Expected total to be %s, got %s", expectedTotal, total))
	})

	// Test with EUR as main currency
	c.Run("EUR as main currency", func(c *qt.C) {
		// Setup test registry with EUR as main currency
		registrySet := setupTestRegistry(c, "EUR")
		valuator := valuation.NewValuator(registrySet)

		// Calculate global total value
		total, err := valuator.CalculateGlobalTotalValue()
		c.Assert(err, qt.IsNil)

		// Expected total: 100 + 300 + 400 = 800 (prices already represent total value for all items)
		expectedTotal := decimal.NewFromFloat(800.00)
		c.Assert(total.Equal(expectedTotal), qt.IsTrue, qt.Commentf("Expected total to be %s, got %s", expectedTotal, total))
	})
}

func TestValuator_CalculateTotalValueByLocation(t *testing.T) {
	c := qt.New(t)

	// Test with USD as main currency
	c.Run("USD as main currency", func(c *qt.C) {
		// Setup test registry with USD as main currency
		registrySet := setupTestRegistry(c, "USD")
		valuator := valuation.NewValuator(registrySet)

		userCtx := appctx.WithUser(c.Context(), &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: "test-tenant-id",
				EntityID: models.EntityID{ID: "test-user-id"},
			},
		})

		locations, err := registrySet.LocationRegistry.List(userCtx)
		c.Assert(err, qt.IsNil)

		locationsByName := map[string]string{
			locations[0].Name: locations[0].ID,
			locations[1].Name: locations[1].ID,
		}

		_ = locationsByName

		// Calculate total value by location
		locationTotals, err := valuator.CalculateTotalValueByLocation()
		c.Assert(err, qt.IsNil)

		// Expected totals:
		// Location 1: 100 + 200 = 300
		// Location 2: 330 + 0 = 330 (Commodity 4 has no valid price in USD)
		expectedTotals := map[string]decimal.Decimal{
			"Location 1": decimal.NewFromFloat(100.00),
			"Location 2": decimal.NewFromFloat(700.00),
		}

		// Check that we have the expected number of totals
		c.Assert(locationTotals, qt.HasLen, len(expectedTotals), qt.Commentf("Expected %d location totals, got %d", len(expectedTotals), len(locationTotals)))

		// Check values by location ID
		for locationName, expectedValue := range expectedTotals {
			locationID, ok := locationsByName[locationName]
			c.Assert(ok, qt.IsTrue, qt.Commentf("Expected to find location with Name %s", locationName))

			actualValue, ok := locationTotals[locationID]
			c.Assert(ok, qt.IsTrue, qt.Commentf("Expected to find location with ID %s", locationID))

			c.Assert(actualValue.Equal(expectedValue), qt.IsTrue,
				qt.Commentf("Expected Location %s value to be %s, got %s", locationID, expectedValue, actualValue))
		}
	})
}

func TestValuator_CalculateTotalValueByArea(t *testing.T) {
	c := qt.New(t)

	// Test with USD as main currency
	c.Run("USD as main currency", func(c *qt.C) {
		// Setup test registry with USD as main currency
		registrySet := setupTestRegistry(c, "USD")
		valuator := valuation.NewValuator(registrySet)

		// Calculate total value by area
		areaTotals, err := valuator.CalculateTotalValueByArea()
		c.Assert(err, qt.IsNil)

		// Get area IDs from the registry
		areas, err := registrySet.AreaRegistry.List(c.Context())
		c.Assert(err, qt.IsNil)
		var area1ID, area2ID string
		for _, area := range areas {
			switch area.Name {
			case "Area 1":
				area1ID = area.ID
			case "Area 2":
				area2ID = area.ID
			}
		}

		// Expected totals:
		// Area 1: 100 + 200 = 300
		// Area 2: 330 = 330
		// Area 3: (0) = 0 (No valid commodities in Area 3 with USD as main currency)
		expectedTotals := map[string]decimal.Decimal{
			area1ID: decimal.NewFromFloat(100.00),
			area2ID: decimal.NewFromFloat(700.00),
		}

		// Check that we have the expected number of totals
		c.Assert(areaTotals, qt.HasLen, len(expectedTotals), qt.Commentf("Expected %d area totals, got %d", len(expectedTotals), len(areaTotals)))

		// Check values by area ID
		for areaID, expectedValue := range expectedTotals {
			actualValue, ok := areaTotals[areaID]
			c.Assert(ok, qt.IsTrue, qt.Commentf("Expected to find area with ID %s", areaID))
			if ok {
				c.Assert(actualValue.Equal(expectedValue), qt.IsTrue,
					qt.Commentf("Expected Area %s value to be %s, got %s", areaID, expectedValue, actualValue))
			}
		}
	})
}
