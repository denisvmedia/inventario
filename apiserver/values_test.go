package apiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func setupValuesTestData(t *testing.T) *registry.Set {
	t.Helper()

	// Create a memory registry for testing
	registrySet, err := memory.NewRegistrySet("")
	require.NoError(t, err)

	// Set main currency to USD
	mainCurrency := "USD"
	err = registrySet.SettingsRegistry.Save(models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	require.NoError(t, err)

	// Create a location
	location, err := registrySet.LocationRegistry.Create(models.Location{
		Name:    "Test Location",
		Address: "123 Test St",
	})
	require.NoError(t, err)

	// Create an area
	area, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	require.NoError(t, err)

	// Create a commodity
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  2,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.NewFromFloat(0.00),
		CurrentPrice:           decimal.NewFromFloat(0.00),
		Status:                 models.CommodityStatusInUse,
		Draft:                  false,
	})
	require.NoError(t, err)

	return registrySet
}

func TestValuesAPI_GetValues(t *testing.T) {
	// Setup test data
	registrySet := setupValuesTestData(t)

	// Create a router with the values endpoint
	r := chi.NewRouter()
	r.Route("/values", Values(registrySet))

	// Test GET /values
	req := httptest.NewRequest("GET", "/values", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Parse response
	var response jsonapi.ValueResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check global total
	expectedTotal := decimal.NewFromFloat(100.00) // Price already represents total value for all items
	assert.True(t, expectedTotal.Equal(response.Data.Attributes.GlobalTotal),
		"Expected global total to be %s, got %s", expectedTotal, response.Data.Attributes.GlobalTotal)

	// Check location totals
	assert.Len(t, response.Data.Attributes.LocationTotals, 1)
	// Get the location ID
	locations, err := registrySet.LocationRegistry.List()
	require.NoError(t, err)
	var locationID string
	for _, loc := range locations {
		if loc.Name == "Test Location" {
			locationID = loc.ID
			break
		}
	}
	require.NotEmpty(t, locationID, "Could not find Test Location")

	// Check the location total
	actualValue, ok := response.Data.Attributes.LocationTotals[locationID]
	assert.True(t, ok, "Expected to find location with ID %s", locationID)
	assert.True(t, expectedTotal.Equal(actualValue),
		"Expected location total to be %s, got %s", expectedTotal, actualValue)

	// Check area totals
	assert.Len(t, response.Data.Attributes.AreaTotals, 1)
	// Get the area ID
	areas, err := registrySet.AreaRegistry.List()
	require.NoError(t, err)
	var areaID string
	for _, area := range areas {
		if area.Name == "Test Area" {
			areaID = area.ID
			break
		}
	}
	require.NotEmpty(t, areaID, "Could not find Test Area")

	// Check the area total
	actualValue, ok = response.Data.Attributes.AreaTotals[areaID]
	assert.True(t, ok, "Expected to find area with ID %s", areaID)
	assert.True(t, expectedTotal.Equal(actualValue),
		"Expected area total to be %s, got %s", expectedTotal, actualValue)
}

func TestValuesAPI_GetDetailedValues(t *testing.T) {
	// Setup test data
	registrySet := setupValuesTestData(t)

	// Create a router with the values endpoint
	r := chi.NewRouter()
	r.Route("/values", Values(registrySet))

	// Test GET /values/detailed
	req := httptest.NewRequest("GET", "/values/detailed", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Parse response
	var response jsonapi.DetailedValueResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check commodity values
	assert.Len(t, response.Data, 1)
	assert.Equal(t, "Test Commodity", response.Data[0].Attributes.Name)

	expectedValue := decimal.NewFromFloat(100.00) // Price already represents total value for all items
	assert.True(t, expectedValue.Equal(response.Data[0].Attributes.Value),
		"Expected commodity value to be %s, got %s", expectedValue, response.Data[0].Attributes.Value)
}
