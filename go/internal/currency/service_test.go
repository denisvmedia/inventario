package currency_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestConversionService_ConvertCommodityPrices(t *testing.T) {
	ctx := validationctx.WithMainCurrency(context.Background(), "USD")

	tests := []struct {
		name         string
		fromCurrency string
		toCurrency   string
		rate         decimal.Decimal
		commodities  []models.Commodity
		expected     []models.Commodity
	}{
		{
			name:         "same currency no conversion",
			fromCurrency: "USD",
			toCurrency:   "USD",
			rate:         decimal.NewFromInt(1),
			commodities: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(100),
					OriginalPriceCurrency:  "USD",
					ConvertedOriginalPrice: decimal.Zero,
					CurrentPrice:           decimal.NewFromInt(90),
				},
			},
			expected: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(100),
					OriginalPriceCurrency:  "USD",
					ConvertedOriginalPrice: decimal.Zero,
					CurrentPrice:           decimal.NewFromInt(90),
				},
			},
		},
		{
			name:         "USD to EUR conversion",
			fromCurrency: "USD",
			toCurrency:   "EUR",
			rate:         decimal.NewFromFloat(0.85), // 1 USD = 0.85 EUR
			commodities: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(100),
					OriginalPriceCurrency:  "USD",
					ConvertedOriginalPrice: decimal.Zero,
					CurrentPrice:           decimal.NewFromInt(90),
				},
			},
			expected: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(85),     // 100 * 0.85
					OriginalPriceCurrency:  "EUR",                      // Changed to new main currency
					ConvertedOriginalPrice: decimal.Zero,               // Should remain zero
					CurrentPrice:           decimal.NewFromFloat(76.5), // 90 * 0.85
				},
			},
		},
		{
			name:         "conversion with non-main currency original price",
			fromCurrency: "USD",
			toCurrency:   "EUR",
			rate:         decimal.NewFromFloat(0.85), // 1 USD = 0.85 EUR
			commodities: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(100),
					OriginalPriceCurrency:  "GBP",                   // Different from main currency
					ConvertedOriginalPrice: decimal.NewFromInt(130), // 100 GBP = 130 USD
					CurrentPrice:           decimal.NewFromInt(90),
				},
			},
			expected: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(100),     // Unchanged
					OriginalPriceCurrency:  "GBP",                       // Unchanged
					ConvertedOriginalPrice: decimal.NewFromFloat(110.5), // 130 * 0.85
					CurrentPrice:           decimal.NewFromFloat(76.5),  // 90 * 0.85
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create registry and service
			registrySet, err := memory.NewRegistrySet("")
			c.Assert(err, qt.IsNil)

			// Create test location and area
			location, err := registrySet.LocationRegistry.Create(ctx, models.Location{
				Name: "Test Location",
			})
			c.Assert(err, qt.IsNil)

			area, err := registrySet.AreaRegistry.Create(ctx, models.Area{
				Name:       "Test Area",
				LocationID: location.ID,
			})
			c.Assert(err, qt.IsNil)

			// Create test commodities
			for _, commodity := range tt.commodities {
				commodity.AreaID = area.ID
				_, err = registrySet.CommodityRegistry.Create(ctx, commodity)
				c.Assert(err, qt.IsNil)
			}

			// Create rate provider
			rateProvider := currency.NewStaticRateProvider(map[string]decimal.Decimal{
				"USD_EUR": tt.rate,
			})

			// Create conversion service
			service := currency.NewConversionService(registrySet.CommodityRegistry, rateProvider)

			// Perform conversion
			err = service.ConvertCommodityPrices(ctx, tt.fromCurrency, tt.toCurrency)
			c.Assert(err, qt.IsNil)

			// Verify results
			updatedCommodities, err := registrySet.CommodityRegistry.List(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(len(updatedCommodities), qt.Equals, len(tt.expected))

			for i, expected := range tt.expected {
				actual := updatedCommodities[i]
				c.Assert(actual.OriginalPrice.Equal(expected.OriginalPrice), qt.IsTrue, qt.Commentf("OriginalPrice: expected %v, got %v", expected.OriginalPrice, actual.OriginalPrice))
				c.Assert(actual.OriginalPriceCurrency, qt.Equals, expected.OriginalPriceCurrency)
				c.Assert(actual.ConvertedOriginalPrice.Equal(expected.ConvertedOriginalPrice), qt.IsTrue, qt.Commentf("ConvertedOriginalPrice: expected %v, got %v", expected.ConvertedOriginalPrice, actual.ConvertedOriginalPrice))
				c.Assert(actual.CurrentPrice.Equal(expected.CurrentPrice), qt.IsTrue, qt.Commentf("CurrentPrice: expected %v, got %v", expected.CurrentPrice, actual.CurrentPrice))
			}
		})
	}
}

func TestConversionService_ConvertCommodityPricesWithRate(t *testing.T) {
	ctx := validationctx.WithMainCurrency(context.Background(), "USD")

	tests := []struct {
		name         string
		fromCurrency string
		toCurrency   string
		customRate   *decimal.Decimal
		commodities  []models.Commodity
		expected     []models.Commodity
	}{
		{
			name:         "USD to EUR with custom rate",
			fromCurrency: "USD",
			toCurrency:   "EUR",
			customRate:   func() *decimal.Decimal { r := decimal.NewFromFloat(0.90); return &r }(), // Custom rate: 1 USD = 0.90 EUR
			commodities: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(100),
					OriginalPriceCurrency:  "USD",
					ConvertedOriginalPrice: decimal.Zero,
					CurrentPrice:           decimal.NewFromInt(90),
				},
			},
			expected: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(90), // 100 * 0.90
					OriginalPriceCurrency:  "EUR",                  // Changed to new main currency
					ConvertedOriginalPrice: decimal.Zero,           // Should remain zero
					CurrentPrice:           decimal.NewFromInt(81), // 90 * 0.90
				},
			},
		},
		{
			name:         "USD to EUR with custom rate and non-main currency original price",
			fromCurrency: "USD",
			toCurrency:   "EUR",
			customRate:   func() *decimal.Decimal { r := decimal.NewFromFloat(0.90); return &r }(), // Custom rate: 1 USD = 0.90 EUR
			commodities: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(100),
					OriginalPriceCurrency:  "GBP",                   // Different from main currency
					ConvertedOriginalPrice: decimal.NewFromInt(130), // 100 GBP = 130 USD
					CurrentPrice:           decimal.NewFromInt(120),
				},
			},
			expected: []models.Commodity{
				{
					EntityID:               models.EntityID{ID: "1"},
					Name:                   "Test Item",
					ShortName:              "TI",
					Type:                   models.CommodityTypeElectronics,
					Status:                 models.CommodityStatusInUse,
					Count:                  1,
					OriginalPrice:          decimal.NewFromInt(100), // Unchanged
					OriginalPriceCurrency:  "GBP",                   // Unchanged
					ConvertedOriginalPrice: decimal.NewFromInt(117), // 130 * 0.90
					CurrentPrice:           decimal.NewFromInt(108), // 120 * 0.90
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Create registry and service
			registrySet, err := memory.NewRegistrySet("")
			c.Assert(err, qt.IsNil)

			// Create test location and area
			location, err := registrySet.LocationRegistry.Create(ctx, models.Location{
				Name: "Test Location",
			})
			c.Assert(err, qt.IsNil)

			area, err := registrySet.AreaRegistry.Create(ctx, models.Area{
				Name:       "Test Area",
				LocationID: location.ID,
			})
			c.Assert(err, qt.IsNil)

			// Create test commodities
			for _, commodity := range tt.commodities {
				commodity.AreaID = area.ID
				_, err = registrySet.CommodityRegistry.Create(ctx, commodity)
				c.Assert(err, qt.IsNil)
			}

			// Create rate provider (not used when custom rate is provided)
			rateProvider := currency.NewStaticRateProvider(map[string]decimal.Decimal{
				"USD_EUR": decimal.NewFromFloat(0.85), // This should not be used
			})

			// Create conversion service
			service := currency.NewConversionService(registrySet.CommodityRegistry, rateProvider)

			// Perform conversion with custom rate
			err = service.ConvertCommodityPricesWithRate(ctx, tt.fromCurrency, tt.toCurrency, tt.customRate)
			c.Assert(err, qt.IsNil)

			// Verify results
			updatedCommodities, err := registrySet.CommodityRegistry.List(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(len(updatedCommodities), qt.Equals, len(tt.expected))

			for i, expected := range tt.expected {
				actual := updatedCommodities[i]
				c.Assert(actual.OriginalPrice.Equal(expected.OriginalPrice), qt.IsTrue, qt.Commentf("OriginalPrice: expected %v, got %v", expected.OriginalPrice, actual.OriginalPrice))
				c.Assert(actual.OriginalPriceCurrency, qt.Equals, expected.OriginalPriceCurrency)
				c.Assert(actual.ConvertedOriginalPrice.Equal(expected.ConvertedOriginalPrice), qt.IsTrue, qt.Commentf("ConvertedOriginalPrice: expected %v, got %v", expected.ConvertedOriginalPrice, actual.ConvertedOriginalPrice))
				c.Assert(actual.CurrentPrice.Equal(expected.CurrentPrice), qt.IsTrue, qt.Commentf("CurrentPrice: expected %v, got %v", expected.CurrentPrice, actual.CurrentPrice))
			}
		})
	}
}

func TestStaticRateProvider_GetExchangeRate(t *testing.T) {
	tests := []struct {
		name     string
		rates    map[string]decimal.Decimal
		from     string
		to       string
		expected decimal.Decimal
		wantErr  bool
	}{
		{
			name: "same currency",
			rates: map[string]decimal.Decimal{
				"USD_EUR": decimal.NewFromFloat(0.85),
			},
			from:     "USD",
			to:       "USD",
			expected: decimal.NewFromInt(1),
			wantErr:  false,
		},
		{
			name: "existing rate",
			rates: map[string]decimal.Decimal{
				"USD_EUR": decimal.NewFromFloat(0.85),
			},
			from:     "USD",
			to:       "EUR",
			expected: decimal.NewFromFloat(0.85),
			wantErr:  false,
		},
		{
			name: "non-existing rate",
			rates: map[string]decimal.Decimal{
				"USD_EUR": decimal.NewFromFloat(0.85),
			},
			from:     "EUR",
			to:       "GBP",
			expected: decimal.Zero,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			provider := currency.NewStaticRateProvider(tt.rates)
			rate, err := provider.GetExchangeRate(context.Background(), tt.from, tt.to)

			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(rate.Equal(tt.expected), qt.IsTrue, qt.Commentf("expected %v, got %v", tt.expected, rate))
			}
		})
	}
}
