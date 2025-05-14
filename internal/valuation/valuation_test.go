package valuation

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// setupTestRegistry creates a test registry with test data
func setupTestRegistry(c *qt.C, mainCurrency string) *registry.Set {
	// Create a memory registry for testing
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Set main currency
	err = registrySet.SettingsRegistry.Save(models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	// Create locations
	location1, err := registrySet.LocationRegistry.Create(models.Location{
		Name: "Location 1",
	})
	c.Assert(err, qt.IsNil)

	location2, err := registrySet.LocationRegistry.Create(models.Location{
		Name: "Location 2",
	})
	c.Assert(err, qt.IsNil)

	// Create areas
	area1, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Area 1",
		LocationID: location1.ID,
	})
	c.Assert(err, qt.IsNil)

	area2, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Area 2",
		LocationID: location2.ID,
	})
	c.Assert(err, qt.IsNil)

	area3, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Area 3",
		LocationID: location2.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create commodities
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:         "Commodity 1",
		ShortName:    "C1",
		AreaID:       area1.ID,
		Count:        2,
		CurrentPrice: decimal.NewFromFloat(100.00),
		Status:       models.CommodityStatusInUse,
		Draft:        false,
		Type:         models.CommodityTypeElectronics,
		PurchaseDate: models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                  "Commodity 2",
		ShortName:             "C2",
		AreaID:                area1.ID,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(200.00),
		OriginalPriceCurrency: "USD",
		Status:                models.CommodityStatusInUse,
		Draft:                 false,
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Commodity 3",
		ShortName:              "C3",
		AreaID:                 area2.ID,
		Count:                  3,
		OriginalPrice:          decimal.NewFromFloat(300.00),
		OriginalPriceCurrency:  "EUR",
		ConvertedOriginalPrice: decimal.NewFromFloat(330.00),
		Status:                 models.CommodityStatusInUse,
		Draft:                  false,
		Type:                   models.CommodityTypeElectronics,
		PurchaseDate:           models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                  "Commodity 4",
		ShortName:             "C4",
		AreaID:                area2.ID,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(400.00),
		OriginalPriceCurrency: "EUR",
		Status:                models.CommodityStatusInUse,
		Draft:                 false,
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:         "Commodity 5",
		ShortName:    "C5",
		AreaID:       area3.ID,
		Count:        1,
		CurrentPrice: decimal.NewFromFloat(500.00),
		Status:       models.CommodityStatusSold, // Not in use
		Draft:        false,
		Type:         models.CommodityTypeElectronics,
		PurchaseDate: models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:         "Commodity 6",
		ShortName:    "C6",
		AreaID:       area3.ID,
		Count:        1,
		CurrentPrice: decimal.NewFromFloat(600.00),
		Status:       models.CommodityStatusInUse,
		Draft:        true, // Draft
		Type:         models.CommodityTypeElectronics,
		PurchaseDate: models.ToPDate("2023-01-01"),
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
		valuator := NewValuator(registrySet)

		// Calculate global total value
		total, err := valuator.CalculateGlobalTotalValue()
		c.Assert(err, qt.IsNil)

		// Expected total: 100 + 200 + 330 = 630 (prices already represent total value for all items)
		expectedTotal := decimal.NewFromFloat(630.00)
		c.Assert(total.Equal(expectedTotal), qt.IsTrue, qt.Commentf("Expected total to be %s, got %s", expectedTotal, total))
	})

	// Test with EUR as main currency
	c.Run("EUR as main currency", func(c *qt.C) {
		// Setup test registry with EUR as main currency
		registrySet := setupTestRegistry(c, "EUR")
		valuator := NewValuator(registrySet)

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
		valuator := NewValuator(registrySet)

		locations, err := registrySet.LocationRegistry.List()
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
			"Location 1": decimal.NewFromFloat(300.00),
			"Location 2": decimal.NewFromFloat(330.00),
		}

		// Check that we have the expected number of totals
		c.Assert(len(locationTotals), qt.Equals, len(expectedTotals), qt.Commentf("Expected %d location totals, got %d", len(expectedTotals), len(locationTotals)))

		// Check values by location ID
		for locationName, expectedValue := range expectedTotals {
			locationID, ok := locationsByName[locationName]
			c.Assert(err, qt.IsNil, qt.Commentf("Expected to find location with Name %s", locationName))

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
		valuator := NewValuator(registrySet)

		// Calculate total value by area
		areaTotals, err := valuator.CalculateTotalValueByArea()
		c.Assert(err, qt.IsNil)

		// Get area IDs from the registry
		areas, err := registrySet.AreaRegistry.List()
		c.Assert(err, qt.IsNil)
		var area1ID, area2ID string
		for _, area := range areas {
			if area.Name == "Area 1" {
				area1ID = area.ID
			} else if area.Name == "Area 2" {
				area2ID = area.ID
			}
		}

		// Expected totals:
		// Area 1: 100 + 200 = 300
		// Area 2: 330 = 330
		// Area 3: (0) = 0 (No valid commodities in Area 3 with USD as main currency)
		expectedTotals := map[string]decimal.Decimal{
			area1ID: decimal.NewFromFloat(300.00),
			area2ID: decimal.NewFromFloat(330.00),
		}

		// Check that we have the expected number of totals
		c.Assert(len(areaTotals), qt.Equals, len(expectedTotals), qt.Commentf("Expected %d area totals, got %d", len(expectedTotals), len(areaTotals)))

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

func TestGetCommodityValue(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name         string
		commodity    *models.Commodity
		mainCurrency string
		expected     decimal.Decimal
	}{
		{
			name: "Current price available",
			commodity: &models.Commodity{
				CurrentPrice:           decimal.NewFromFloat(100.00),
				OriginalPrice:          decimal.NewFromFloat(200.00),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.NewFromFloat(220.00),
			},
			mainCurrency: "USD",
			expected:     decimal.NewFromFloat(100.00),
		},
		{
			name: "No current price, original price in main currency",
			commodity: &models.Commodity{
				CurrentPrice:           decimal.NewFromFloat(0.00),
				OriginalPrice:          decimal.NewFromFloat(200.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.NewFromFloat(0.00),
			},
			mainCurrency: "USD",
			expected:     decimal.NewFromFloat(200.00),
		},
		{
			name: "No current price, original price not in main currency, converted price available",
			commodity: &models.Commodity{
				CurrentPrice:           decimal.NewFromFloat(0.00),
				OriginalPrice:          decimal.NewFromFloat(200.00),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.NewFromFloat(220.00),
			},
			mainCurrency: "USD",
			expected:     decimal.NewFromFloat(220.00),
		},
		{
			name: "No current price, original price not in main currency, no converted price",
			commodity: &models.Commodity{
				CurrentPrice:           decimal.NewFromFloat(0.00),
				OriginalPrice:          decimal.NewFromFloat(200.00),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.NewFromFloat(0.00),
			},
			mainCurrency: "USD",
			expected:     decimal.NewFromFloat(0.00),
		},
		{
			name: "No prices at all",
			commodity: &models.Commodity{
				CurrentPrice:           decimal.NewFromFloat(0.00),
				OriginalPrice:          decimal.NewFromFloat(0.00),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.NewFromFloat(0.00),
			},
			mainCurrency: "USD",
			expected:     decimal.NewFromFloat(0.00),
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			result := getCommodityValue(tt.commodity, tt.mainCurrency)
			c.Assert(result.Equal(tt.expected), qt.IsTrue,
				qt.Commentf("Expected %s, got %s", tt.expected, result))
		})
	}
}
