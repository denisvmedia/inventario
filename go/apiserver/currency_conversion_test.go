package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestCurrencyConversionWithCommodities(t *testing.T) {
	c := qt.New(t)

	// Create a memory registry for testing
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Create a conversion service for testing
	rateProvider := currency.NewStaticRateProvider(map[string]decimal.Decimal{
		"USD_EUR": decimal.NewFromFloat(0.85), // 1 USD = 0.85 EUR
		"EUR_USD": decimal.NewFromFloat(1.18),
	})
	conversionService := currency.NewConversionService(registrySet.CommodityRegistry, rateProvider)

	// Create a router with the settings endpoint
	r := chi.NewRouter()
	r.Route("/settings", apiserver.Settings(registrySet.SettingsRegistry, conversionService))

	ctx := validationctx.WithMainCurrency(context.Background(), "USD")

	// Create test data: location and area
	location, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Test Location",
	})
	c.Assert(err, qt.IsNil)

	area, err := registrySet.AreaRegistry.Create(ctx, models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create a commodity with USD prices
	purchaseDate := models.Date("2023-01-01")
	commodity := models.Commodity{
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		Status:                 models.CommodityStatusInUse,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromInt(100), // $100 original price
		OriginalPriceCurrency:  "USD",                   // in USD
		ConvertedOriginalPrice: decimal.Zero,            // zero because original is in main currency
		CurrentPrice:           decimal.NewFromInt(90),  // $90 current price
		PurchaseDate:           &purchaseDate,           // required for non-draft
	}

	_, err = registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Set main currency to USD first
	mainCurrency := "USD"
	testSettings := apiserver.SettingsUpdateRequest{
		SettingsObject: models.SettingsObject{
			MainCurrency: &mainCurrency,
		},
	}
	settingsJSON, err := json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Change main currency to EUR
	newCurrency := "EUR"
	testSettings.MainCurrency = &newCurrency
	settingsJSON, err = json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Verify the commodity prices were converted
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)

	updatedCommodity := commodities[0]

	// Original price should be converted to EUR and currency changed
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.NewFromInt(85)), qt.IsTrue, qt.Commentf("OriginalPrice: expected 85, got %v", updatedCommodity.OriginalPrice))
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency("EUR"))

	// ConvertedOriginalPrice should be zero (since original is now in main currency)
	c.Assert(updatedCommodity.ConvertedOriginalPrice.Equal(decimal.Zero), qt.IsTrue)

	// Current price should be converted to EUR
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.NewFromFloat(76.5)), qt.IsTrue, qt.Commentf("CurrentPrice: expected 76.5, got %v", updatedCommodity.CurrentPrice))
}

func TestCurrencyConversionWithCustomExchangeRate(t *testing.T) {
	c := qt.New(t)

	// Create a memory registry for testing
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Create a conversion service for testing
	rateProvider := currency.NewStaticRateProvider(map[string]decimal.Decimal{
		"USD_EUR": decimal.NewFromFloat(0.85), // Default rate, should not be used
		"EUR_USD": decimal.NewFromFloat(1.18),
	})
	conversionService := currency.NewConversionService(registrySet.CommodityRegistry, rateProvider)

	// Create a router with the settings endpoint
	r := chi.NewRouter()
	r.Route("/settings", apiserver.Settings(registrySet.SettingsRegistry, conversionService))

	ctx := validationctx.WithMainCurrency(context.Background(), "USD")

	// Create test data: location and area
	location, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Test Location",
	})
	c.Assert(err, qt.IsNil)

	area, err := registrySet.AreaRegistry.Create(ctx, models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create a commodity with USD prices
	purchaseDate := models.Date("2023-01-01")
	commodity := models.Commodity{
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		Status:                 models.CommodityStatusInUse,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromInt(100), // $100 original price
		OriginalPriceCurrency:  "USD",                   // in USD
		ConvertedOriginalPrice: decimal.Zero,            // zero because original is in main currency
		CurrentPrice:           decimal.NewFromInt(90),  // $90 current price
		PurchaseDate:           &purchaseDate,           // required for non-draft
	}

	_, err = registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Set main currency to USD first
	mainCurrency := "USD"
	testSettings := apiserver.SettingsUpdateRequest{
		SettingsObject: models.SettingsObject{
			MainCurrency: &mainCurrency,
		},
	}
	settingsJSON, err := json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Change main currency to EUR with custom exchange rate
	newCurrency := "EUR"
	customRate := decimal.NewFromFloat(0.90) // Custom rate: 1 USD = 0.90 EUR
	testSettings.MainCurrency = &newCurrency
	testSettings.ExchangeRate = &customRate
	settingsJSON, err = json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Verify the commodity prices were converted using custom rate
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)

	updatedCommodity := commodities[0]

	// Original price should be converted to EUR using custom rate: 100 * 0.90 = 90
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.NewFromInt(90)), qt.IsTrue, qt.Commentf("OriginalPrice: expected 90, got %v", updatedCommodity.OriginalPrice))
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency("EUR"))

	// ConvertedOriginalPrice should be zero (since original is now in main currency)
	c.Assert(updatedCommodity.ConvertedOriginalPrice.Equal(decimal.Zero), qt.IsTrue)

	// Current price should be converted to EUR using custom rate: 90 * 0.90 = 81
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.NewFromInt(81)), qt.IsTrue, qt.Commentf("CurrentPrice: expected 81, got %v", updatedCommodity.CurrentPrice))
}

func TestCurrencyConversionWithCustomExchangeRateUsingPatch(t *testing.T) {
	c := qt.New(t)

	// Create a memory registry for testing
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Create a conversion service for testing
	rateProvider := currency.NewStaticRateProvider(map[string]decimal.Decimal{
		"USD_EUR": decimal.NewFromFloat(0.85), // Default rate, should not be used
		"EUR_USD": decimal.NewFromFloat(1.18),
	})
	conversionService := currency.NewConversionService(registrySet.CommodityRegistry, rateProvider)

	// Create a router with the settings endpoint
	r := chi.NewRouter()
	r.Route("/settings", apiserver.Settings(registrySet.SettingsRegistry, conversionService))

	ctx := validationctx.WithMainCurrency(context.Background(), "USD")

	// Create test data: location and area
	location, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Test Location",
	})
	c.Assert(err, qt.IsNil)

	area, err := registrySet.AreaRegistry.Create(ctx, models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create a commodity with USD prices
	purchaseDate := models.Date("2023-01-01")
	commodity := models.Commodity{
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		Status:                 models.CommodityStatusInUse,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromInt(100), // $100 original price
		OriginalPriceCurrency:  "USD",                   // in USD
		ConvertedOriginalPrice: decimal.Zero,            // zero because original is in main currency
		CurrentPrice:           decimal.NewFromInt(90),  // $90 current price
		PurchaseDate:           &purchaseDate,           // required for non-draft
	}

	_, err = registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Set main currency to USD first
	mainCurrency := "USD"
	testSettings := apiserver.SettingsUpdateRequest{
		SettingsObject: models.SettingsObject{
			MainCurrency: &mainCurrency,
		},
	}
	settingsJSON, err := json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Change main currency to EUR with custom exchange rate using PATCH
	newCurrency := "EUR"
	customRate := decimal.NewFromFloat(0.95) // Custom rate: 1 USD = 0.95 EUR
	patchRequest := apiserver.PatchSettingRequest{
		Value:        newCurrency,
		ExchangeRate: &customRate,
	}
	patchJSON, err := json.Marshal(patchRequest)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PATCH", "/settings/system.main_currency", bytes.NewReader(patchJSON))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Verify the commodity prices were converted using custom rate
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)

	updatedCommodity := commodities[0]

	// Original price should be converted to EUR using custom rate: 100 * 0.95 = 95
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.NewFromInt(95)), qt.IsTrue, qt.Commentf("OriginalPrice: expected 95, got %v", updatedCommodity.OriginalPrice))
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency("EUR"))

	// ConvertedOriginalPrice should be zero (since original is now in main currency)
	c.Assert(updatedCommodity.ConvertedOriginalPrice.Equal(decimal.Zero), qt.IsTrue)

	// Current price should be converted to EUR using custom rate: 90 * 0.95 = 85.5
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.NewFromFloat(85.5)), qt.IsTrue, qt.Commentf("CurrentPrice: expected 85.5, got %v", updatedCommodity.CurrentPrice))
}

func TestCurrencyConversionWithNonMainCurrencyOriginalPrice(t *testing.T) {
	c := qt.New(t)

	// Create a memory registry for testing
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Create a conversion service for testing
	rateProvider := currency.NewStaticRateProvider(map[string]decimal.Decimal{
		"USD_EUR": decimal.NewFromFloat(0.85), // 1 USD = 0.85 EUR
		"EUR_USD": decimal.NewFromFloat(1.18),
	})
	conversionService := currency.NewConversionService(registrySet.CommodityRegistry, rateProvider)

	// Create a router with the settings endpoint
	r := chi.NewRouter()
	r.Route("/settings", apiserver.Settings(registrySet.SettingsRegistry, conversionService))

	ctx := validationctx.WithMainCurrency(context.Background(), "USD")

	// Create test data: location and area
	location, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Test Location",
	})
	c.Assert(err, qt.IsNil)

	area, err := registrySet.AreaRegistry.Create(ctx, models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create a commodity with GBP original price and USD converted/current prices
	purchaseDate := models.Date("2023-01-01")
	commodity := models.Commodity{
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		Status:                 models.CommodityStatusInUse,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromInt(100), // £100 original price
		OriginalPriceCurrency:  "GBP",                   // in GBP
		ConvertedOriginalPrice: decimal.NewFromInt(130), // $130 converted original price
		CurrentPrice:           decimal.NewFromInt(120), // $120 current price
		PurchaseDate:           &purchaseDate,           // required for non-draft
	}

	_, err = registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Set main currency to USD first
	mainCurrency := "USD"
	testSettings := models.SettingsObject{
		MainCurrency: &mainCurrency,
	}
	settingsJSON, err := json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Change main currency to EUR
	newCurrency := "EUR"
	testSettings.MainCurrency = &newCurrency
	settingsJSON, err = json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Verify the commodity prices were converted
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)

	updatedCommodity := commodities[0]

	// Original price should remain unchanged (£100 in GBP)
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.NewFromInt(100)), qt.IsTrue)
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency("GBP"))

	// ConvertedOriginalPrice should be converted from USD to EUR: 130 * 0.85 = 110.5
	c.Assert(updatedCommodity.ConvertedOriginalPrice.Equal(decimal.NewFromFloat(110.5)), qt.IsTrue, qt.Commentf("ConvertedOriginalPrice: expected 110.5, got %v", updatedCommodity.ConvertedOriginalPrice))

	// Current price should be converted from USD to EUR: 120 * 0.85 = 102
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.NewFromFloat(102)), qt.IsTrue, qt.Commentf("CurrentPrice: expected 102, got %v", updatedCommodity.CurrentPrice))
}
