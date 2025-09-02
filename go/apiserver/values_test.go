package apiserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func setupValuesTestData(c *qt.C) (*registry.FactorySet, *models.User) {
	c.Helper()

	// Create a memory factory set for testing
	factorySet := memory.NewFactorySet()
	userRegistry, testUser := newUserRegistryWithUser()
	factorySet.UserRegistry = userRegistry
	c.Assert(factorySet, qt.IsNotNil)

	// Get user-aware registry set
	registrySet := must.Must(factorySet.CreateUserRegistrySet(appctx.WithUser(c.Context(), testUser)))

	// Set main currency to USD
	mainCurrency := "USD"
	err := registrySet.SettingsRegistry.Save(c.Context(), models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	// Create a location
	location, err := registrySet.LocationRegistry.Create(c.Context(), models.Location{
		Name:    "Test Location",
		Address: "123 Test St",
	})
	c.Assert(err, qt.IsNil)

	// Create an area
	area, err := registrySet.AreaRegistry.Create(c.Context(), models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create a commodity
	_, err = registrySet.CommodityRegistry.Create(c.Context(), models.Commodity{
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
	c.Assert(err, qt.IsNil)

	return factorySet, testUser
}

func TestValuesAPI_GetValues(t *testing.T) {
	c := qt.New(t)

	// Setup test data
	factorySet, testUser := setupValuesTestData(c)

	// Create a router with the values endpoint and required middleware
	r := chi.NewRouter()
	r.With(apiserver.RequireAuth(testJWTSecret, factorySet.UserRegistry)).With(apiserver.RegistrySetMiddleware(factorySet)).Route("/values", apiserver.Values())

	// Test GET /values
	req := httptest.NewRequest("GET", "/values", nil)
	addTestUserAuthHeader(req, testUser.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check response
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

	// Parse response
	var response jsonapi.ValueResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)

	// Check global total
	expectedTotal := decimal.NewFromFloat(100.00) // Price already represents total value for all items
	c.Assert(expectedTotal.Equal(response.Data.Attributes.GlobalTotal), qt.IsTrue,
		qt.Commentf("Expected global total to be %s, got %s", expectedTotal, response.Data.Attributes.GlobalTotal))

	// Check location totals
	c.Assert(response.Data.Attributes.LocationTotals, qt.HasLen, 1)
	// Get the location ID
	registrySet := must.Must(factorySet.CreateUserRegistrySet(appctx.WithUser(c.Context(), testUser)))
	locations, err := registrySet.LocationRegistry.List(c.Context())
	c.Assert(err, qt.IsNil)
	var locationID string
	for _, loc := range locations {
		if loc.Name == "Test Location" {
			locationID = loc.ID
			break
		}
	}
	c.Assert(locationID, qt.Not(qt.Equals), "", qt.Commentf("Could not find Test Location"))

	// Check the location total
	actualValue, ok := response.Data.Attributes.LocationTotals[locationID]
	c.Assert(ok, qt.IsTrue, qt.Commentf("Expected to find location with ID %s", locationID))
	c.Assert(expectedTotal.Equal(actualValue), qt.IsTrue,
		qt.Commentf("Expected location total to be %s, got %s", expectedTotal, actualValue))

	// Check area totals
	c.Assert(response.Data.Attributes.AreaTotals, qt.HasLen, 1)
	// Get the area ID
	areas, err := registrySet.AreaRegistry.List(c.Context())
	c.Assert(err, qt.IsNil)
	var areaID string
	for _, area := range areas {
		if area.Name == "Test Area" {
			areaID = area.ID
			break
		}
	}
	c.Assert(areaID, qt.Not(qt.Equals), "", qt.Commentf("Could not find Test Area"))

	// Check the area total
	actualValue, ok = response.Data.Attributes.AreaTotals[areaID]
	c.Assert(ok, qt.IsTrue, qt.Commentf("Expected to find area with ID %s", areaID))
	c.Assert(expectedTotal.Equal(actualValue), qt.IsTrue,
		qt.Commentf("Expected area total to be %s, got %s", expectedTotal, actualValue))
}
