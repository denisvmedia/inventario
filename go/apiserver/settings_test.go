package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestSettingsAPI(t *testing.T) {
	c := qt.New(t)

	// Create a memory registry for testing
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Create a conversion service for testing
	rateProvider := currency.NewStaticRateProvider(map[string]decimal.Decimal{
		"USD_EUR": decimal.NewFromFloat(0.85),
		"EUR_USD": decimal.NewFromFloat(1.18),
		"EUR_GBP": decimal.NewFromFloat(0.86),
		"GBP_EUR": decimal.NewFromFloat(1.16),
	})
	conversionService := currency.NewConversionService(registrySet.CommodityRegistry, rateProvider)

	// Create a router with the settings endpoint
	r := chi.NewRouter()
	r.Route("/settings", apiserver.Settings(registrySet.SettingsRegistry, conversionService))

	// Test GET /settings (empty settings)
	req := httptest.NewRequest("GET", "/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

	var emptySettings models.SettingsObject
	err = json.Unmarshal(w.Body.Bytes(), &emptySettings)
	c.Assert(err, qt.IsNil)
	c.Assert(emptySettings, qt.DeepEquals, models.SettingsObject{})

	// Test PUT /settings
	theme := "dark"
	showDebugInfo := true
	testSettings := models.SettingsObject{
		Theme:         &theme,
		ShowDebugInfo: &showDebugInfo,
	}
	settingsJSON, err := json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

	var updatedSettings models.SettingsObject
	err = json.Unmarshal(w.Body.Bytes(), &updatedSettings)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedSettings, qt.DeepEquals, testSettings)

	// Test GET /settings after PUT
	req = httptest.NewRequest("GET", "/settings", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

	var retrievedSettings models.SettingsObject
	err = json.Unmarshal(w.Body.Bytes(), &retrievedSettings)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedSettings, qt.DeepEquals, testSettings)

	// Test PATCH /settings/{field}
	newTheme := "light"
	themeJSON, err := json.Marshal(newTheme)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PATCH", "/settings/uiconfig.theme", bytes.NewReader(themeJSON))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

	var patchedSettings models.SettingsObject
	err = json.Unmarshal(w.Body.Bytes(), &patchedSettings)
	c.Assert(err, qt.IsNil)
	c.Assert(*patchedSettings.Theme, qt.Equals, newTheme)
	c.Assert(*patchedSettings.ShowDebugInfo, qt.Equals, showDebugInfo)

	// Test GET /settings after PATCH
	req = httptest.NewRequest("GET", "/settings", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

	var finalSettings models.SettingsObject
	err = json.Unmarshal(w.Body.Bytes(), &finalSettings)
	c.Assert(err, qt.IsNil)
	c.Assert(*finalSettings.Theme, qt.Equals, newTheme)
	c.Assert(*finalSettings.ShowDebugInfo, qt.Equals, showDebugInfo)
}

func TestMainCurrencyCanBeChanged(t *testing.T) {
	c := qt.New(t)

	// Create a memory registry for testing
	registrySet, err := memory.NewRegistrySet("")
	c.Assert(err, qt.IsNil)

	// Create a conversion service for testing
	rateProvider := currency.NewStaticRateProvider(map[string]decimal.Decimal{
		"USD_EUR": decimal.NewFromFloat(0.85),
		"EUR_USD": decimal.NewFromFloat(1.18),
		"EUR_GBP": decimal.NewFromFloat(0.86),
		"GBP_EUR": decimal.NewFromFloat(1.16),
	})
	conversionService := currency.NewConversionService(registrySet.CommodityRegistry, rateProvider)

	// Create a router with the settings endpoint
	r := chi.NewRouter()
	r.Route("/settings", apiserver.Settings(registrySet.SettingsRegistry, conversionService))

	// First, set the main currency to USD
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

	// Now try to change the main currency to EUR using PUT
	newCurrency := "EUR"
	testSettings.MainCurrency = &newCurrency
	settingsJSON, err = json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should succeed now
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Try to change the main currency to GBP using PATCH
	finalCurrency := "GBP"
	currencyJSON, err := json.Marshal(finalCurrency)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PATCH", "/settings/system.main_currency", bytes.NewReader(currencyJSON))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should succeed now
	if w.Code != http.StatusOK {
		c.Logf("Response body: %s", w.Body.String())
		c.Logf("Response code: %d", w.Code)
	}
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Verify the main currency is now GBP
	req = httptest.NewRequest("GET", "/settings", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)

	var retrievedSettings models.SettingsObject
	err = json.Unmarshal(w.Body.Bytes(), &retrievedSettings)
	c.Assert(err, qt.IsNil)
	c.Assert(*retrievedSettings.MainCurrency, qt.Equals, finalCurrency) // Should be GBP
}
