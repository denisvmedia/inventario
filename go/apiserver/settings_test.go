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
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

type settingsUpdatePayload struct {
	models.SettingsObject
	ExchangeRate decimal.Decimal `json:"exchange_rate"`
}

type patchSettingPayload struct {
	Value        string          `json:"value"`
	ExchangeRate decimal.Decimal `json:"exchange_rate"`
}

func TestSettingsAPI(t *testing.T) {
	c := qt.New(t)

	// Create a memory factory set for testing
	factorySet := memory.NewFactorySet()
	c.Assert(factorySet, qt.IsNotNil)

	// Create a router with the settings endpoint and registry middleware
	r := chi.NewRouter()
	r.Use(apiserver.RegistrySetMiddleware(factorySet))
	r.Route("/settings", apiserver.Settings())

	// Test GET /settings (empty settings)
	req := httptest.NewRequest("GET", "/settings", nil)
	req = req.WithContext(appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Header().Get("Content-Type"), qt.Equals, "application/json")

	var emptySettings models.SettingsObject
	err := json.Unmarshal(w.Body.Bytes(), &emptySettings)
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
	req = req.WithContext(appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}))
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
	req = req.WithContext(appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}))
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
	req = req.WithContext(appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}))
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
	req = req.WithContext(appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}))
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

func TestSettingsAPI_UpdateMainCurrency_UsesDefaultRate(t *testing.T) {
	c := qt.New(t)

	env := newSettingsTestEnv(t)
	ctx, registrySet := newUserRegistrySet(t, env.factorySet, "user-default-rate", "tenant-a")
	area := createTestArea(t, ctx, registrySet)

	usd := "USD"
	eur := "EUR"
	err := registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{MainCurrency: &usd})
	c.Assert(err, qt.IsNil)

	sameCurrencyCommodity := createCommodity(t, ctx, registrySet, models.Commodity{
		Name:                   "Laptop",
		ShortName:              "LTP",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("100"),
		OriginalPriceCurrency:  models.Currency(usd),
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.RequireFromString("60"),
		Status:                 models.CommodityStatusInUse,
	})
	convertedCommodity := createCommodity(t, ctx, registrySet, models.Commodity{
		Name:                   "Amplifier",
		ShortName:              "AMP",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("200"),
		OriginalPriceCurrency:  models.Currency("GBP"),
		ConvertedOriginalPrice: decimal.RequireFromString("130"),
		CurrentPrice:           decimal.RequireFromString("110"),
		Status:                 models.CommodityStatusInUse,
	})

	response := performSettingsRequest(t, env.router, ctx, http.MethodPut, "/settings", models.SettingsObject{MainCurrency: &eur})
	c.Assert(response.Code, qt.Equals, http.StatusOK)

	updatedSettings := decodeSettingsResponse(t, response)
	c.Assert(updatedSettings.MainCurrency, qt.IsNotNil)
	c.Assert(*updatedSettings.MainCurrency, qt.Equals, eur)

	updatedSameCurrencyCommodity, err := registrySet.CommodityRegistry.Get(ctx, sameCurrencyCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedSameCurrencyCommodity.OriginalPrice.Equal(decimal.RequireFromString("85")), qt.IsTrue)
	c.Assert(updatedSameCurrencyCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(eur))
	c.Assert(updatedSameCurrencyCommodity.ConvertedOriginalPrice.Equal(decimal.Zero), qt.IsTrue)
	c.Assert(updatedSameCurrencyCommodity.CurrentPrice.Equal(decimal.RequireFromString("51")), qt.IsTrue)

	updatedConvertedCommodity, err := registrySet.CommodityRegistry.Get(ctx, convertedCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedConvertedCommodity.OriginalPrice.Equal(decimal.RequireFromString("200")), qt.IsTrue)
	c.Assert(updatedConvertedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency("GBP"))
	c.Assert(updatedConvertedCommodity.ConvertedOriginalPrice.Equal(decimal.RequireFromString("110.5")), qt.IsTrue)
	c.Assert(updatedConvertedCommodity.CurrentPrice.Equal(decimal.RequireFromString("93.5")), qt.IsTrue)
}

func TestSettingsAPI_UpdateMainCurrency_UsesProvidedExchangeRate(t *testing.T) {
	c := qt.New(t)

	env := newSettingsTestEnv(t)
	ctx, registrySet := newUserRegistrySet(t, env.factorySet, "user-put-custom-rate", "tenant-a")
	area := createTestArea(t, ctx, registrySet)

	usd := "USD"
	cad := "CAD"
	err := registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{MainCurrency: &usd})
	c.Assert(err, qt.IsNil)

	commodity := createCommodity(t, ctx, registrySet, models.Commodity{
		Name:                   "Camera",
		ShortName:              "CAM",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("100"),
		OriginalPriceCurrency:  models.Currency(usd),
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.RequireFromString("40"),
		Status:                 models.CommodityStatusInUse,
	})

	response := performSettingsRequest(t, env.router, ctx, http.MethodPut, "/settings", settingsUpdatePayload{
		SettingsObject: models.SettingsObject{MainCurrency: &cad},
		ExchangeRate:   decimal.RequireFromString("1.25"),
	})
	c.Assert(response.Code, qt.Equals, http.StatusOK)

	updatedCommodity, err := registrySet.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.RequireFromString("125")), qt.IsTrue)
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(cad))
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.RequireFromString("50")), qt.IsTrue)
}

func TestSettingsAPI_UpdateMainCurrency_InvalidCurrencyReturnsBadRequest(t *testing.T) {
	c := qt.New(t)

	env := newSettingsTestEnv(t)
	ctx, registrySet := newUserRegistrySet(t, env.factorySet, "user-put-invalid", "tenant-a")
	area := createTestArea(t, ctx, registrySet)

	usd := "USD"
	invalid := "FOO"
	err := registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{MainCurrency: &usd})
	c.Assert(err, qt.IsNil)

	commodity := createCommodity(t, ctx, registrySet, models.Commodity{
		Name:                  "Speaker",
		ShortName:             "SPK",
		Type:                  models.CommodityTypeElectronics,
		AreaID:                area.ID,
		Count:                 1,
		OriginalPrice:         decimal.RequireFromString("100"),
		OriginalPriceCurrency: models.Currency(usd),
		CurrentPrice:          decimal.RequireFromString("60"),
		Status:                models.CommodityStatusInUse,
	})

	response := performSettingsRequest(t, env.router, ctx, http.MethodPut, "/settings", models.SettingsObject{MainCurrency: &invalid})
	c.Assert(response.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(response.Body.String(), qt.Contains, "invalid currency value")

	updatedCommodity, err := registrySet.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.RequireFromString("100")), qt.IsTrue)
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(usd))
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.RequireFromString("60")), qt.IsTrue)

	updatedSettings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedSettings.MainCurrency, qt.IsNotNil)
	c.Assert(*updatedSettings.MainCurrency, qt.Equals, usd)
}

func TestSettingsAPI_PatchMainCurrency_RawStringConvertsCommodity(t *testing.T) {
	c := qt.New(t)

	env := newSettingsTestEnv(t)
	ctx, registrySet := newUserRegistrySet(t, env.factorySet, "user-patch-raw", "tenant-a")
	area := createTestArea(t, ctx, registrySet)

	usd := "USD"
	eur := "EUR"
	err := registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{MainCurrency: &usd})
	c.Assert(err, qt.IsNil)

	commodity := createCommodity(t, ctx, registrySet, models.Commodity{
		Name:                   "Monitor",
		ShortName:              "MON",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("50"),
		OriginalPriceCurrency:  models.Currency(usd),
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.RequireFromString("20"),
		Status:                 models.CommodityStatusInUse,
	})

	response := performSettingsRequest(t, env.router, ctx, http.MethodPatch, "/settings/system.main_currency", eur)
	c.Assert(response.Code, qt.Equals, http.StatusOK)

	updatedSettings := decodeSettingsResponse(t, response)
	c.Assert(updatedSettings.MainCurrency, qt.IsNotNil)
	c.Assert(*updatedSettings.MainCurrency, qt.Equals, eur)

	updatedCommodity, err := registrySet.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.RequireFromString("42.5")), qt.IsTrue)
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(eur))
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.RequireFromString("17")), qt.IsTrue)
}

func TestSettingsAPI_PatchMainCurrency_EnvelopeUsesProvidedExchangeRate(t *testing.T) {
	c := qt.New(t)

	env := newSettingsTestEnv(t)
	ctx, registrySet := newUserRegistrySet(t, env.factorySet, "user-patch-envelope", "tenant-a")
	area := createTestArea(t, ctx, registrySet)

	eur := "EUR"
	chf := "CHF"
	err := registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{MainCurrency: &eur})
	c.Assert(err, qt.IsNil)

	commodity := createCommodity(t, ctx, registrySet, models.Commodity{
		Name:                   "Desk",
		ShortName:              "DSK",
		Type:                   models.CommodityTypeFurniture,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("80"),
		OriginalPriceCurrency:  models.Currency(eur),
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.RequireFromString("20"),
		Status:                 models.CommodityStatusInUse,
	})

	response := performSettingsRequest(t, env.router, ctx, http.MethodPatch, "/settings/system.main_currency", patchSettingPayload{
		Value:        chf,
		ExchangeRate: decimal.RequireFromString("1.50"),
	})
	c.Assert(response.Code, qt.Equals, http.StatusOK)

	updatedCommodity, err := registrySet.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.RequireFromString("120")), qt.IsTrue)
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(chf))
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.RequireFromString("30")), qt.IsTrue)
}

func TestSettingsAPI_PatchMainCurrency_InvalidCurrencyReturnsBadRequest(t *testing.T) {
	c := qt.New(t)

	env := newSettingsTestEnv(t)
	ctx, registrySet := newUserRegistrySet(t, env.factorySet, "user-patch-invalid", "tenant-a")
	area := createTestArea(t, ctx, registrySet)

	usd := "USD"
	invalid := "FOO"
	err := registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{MainCurrency: &usd})
	c.Assert(err, qt.IsNil)

	commodity := createCommodity(t, ctx, registrySet, models.Commodity{
		Name:                  "Projector",
		ShortName:             "PRJ",
		Type:                  models.CommodityTypeElectronics,
		AreaID:                area.ID,
		Count:                 1,
		OriginalPrice:         decimal.RequireFromString("80"),
		OriginalPriceCurrency: models.Currency(usd),
		CurrentPrice:          decimal.RequireFromString("30"),
		Status:                models.CommodityStatusInUse,
	})

	response := performSettingsRequest(t, env.router, ctx, http.MethodPatch, "/settings/system.main_currency", invalid)
	c.Assert(response.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(response.Body.String(), qt.Contains, "invalid currency value")

	updatedCommodity, err := registrySet.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.RequireFromString("80")), qt.IsTrue)
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(usd))
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.RequireFromString("30")), qt.IsTrue)

	updatedSettings, err := registrySet.SettingsRegistry.Get(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedSettings.MainCurrency, qt.IsNotNil)
	c.Assert(*updatedSettings.MainCurrency, qt.Equals, usd)
}

func TestSettingsAPI_PatchMainCurrency_UnchangedCurrencyLeavesCommodityUntouched(t *testing.T) {
	c := qt.New(t)

	env := newSettingsTestEnv(t)
	ctx, registrySet := newUserRegistrySet(t, env.factorySet, "user-patch-unchanged", "tenant-a")
	area := createTestArea(t, ctx, registrySet)

	usd := "USD"
	err := registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{MainCurrency: &usd})
	c.Assert(err, qt.IsNil)

	commodity := createCommodity(t, ctx, registrySet, models.Commodity{
		Name:                   "Phone",
		ShortName:              "PHN",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("90"),
		OriginalPriceCurrency:  models.Currency(usd),
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.RequireFromString("30"),
		Status:                 models.CommodityStatusInUse,
	})

	response := performSettingsRequest(t, env.router, ctx, http.MethodPatch, "/settings/system.main_currency", usd)
	c.Assert(response.Code, qt.Equals, http.StatusOK)

	updatedCommodity, err := registrySet.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedCommodity.OriginalPrice.Equal(decimal.RequireFromString("90")), qt.IsTrue)
	c.Assert(updatedCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(usd))
	c.Assert(updatedCommodity.CurrentPrice.Equal(decimal.RequireFromString("30")), qt.IsTrue)
}

func TestSettingsAPI_MainCurrencyMigration_IsolatedPerUser(t *testing.T) {
	c := qt.New(t)

	env := newSettingsTestEnv(t)
	userOneCtx, userOneRegistrySet := newUserRegistrySet(t, env.factorySet, "user-one", "tenant-a")
	userTwoCtx, userTwoRegistrySet := newUserRegistrySet(t, env.factorySet, "user-two", "tenant-a")
	userOneArea := createTestArea(t, userOneCtx, userOneRegistrySet)
	userTwoArea := createTestArea(t, userTwoCtx, userTwoRegistrySet)

	usd := "USD"
	eur := "EUR"
	err := userOneRegistrySet.SettingsRegistry.Save(userOneCtx, models.SettingsObject{MainCurrency: &usd})
	c.Assert(err, qt.IsNil)
	err = userTwoRegistrySet.SettingsRegistry.Save(userTwoCtx, models.SettingsObject{MainCurrency: &usd})
	c.Assert(err, qt.IsNil)

	userOneCommodity := createCommodity(t, userOneCtx, userOneRegistrySet, models.Commodity{
		Name:                   "User One Item",
		ShortName:              "U1I",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 userOneArea.ID,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("100"),
		OriginalPriceCurrency:  models.Currency(usd),
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.RequireFromString("40"),
		Status:                 models.CommodityStatusInUse,
	})
	userTwoCommodity := createCommodity(t, userTwoCtx, userTwoRegistrySet, models.Commodity{
		Name:                   "User Two Item",
		ShortName:              "U2I",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 userTwoArea.ID,
		Count:                  1,
		OriginalPrice:          decimal.RequireFromString("100"),
		OriginalPriceCurrency:  models.Currency(usd),
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.RequireFromString("40"),
		Status:                 models.CommodityStatusInUse,
	})

	response := performSettingsRequest(t, env.router, userOneCtx, http.MethodPatch, "/settings/system.main_currency", eur)
	c.Assert(response.Code, qt.Equals, http.StatusOK)

	updatedUserOneCommodity, err := userOneRegistrySet.CommodityRegistry.Get(userOneCtx, userOneCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedUserOneCommodity.OriginalPrice.Equal(decimal.RequireFromString("85")), qt.IsTrue)
	c.Assert(updatedUserOneCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(eur))

	updatedUserTwoCommodity, err := userTwoRegistrySet.CommodityRegistry.Get(userTwoCtx, userTwoCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedUserTwoCommodity.OriginalPrice.Equal(decimal.RequireFromString("100")), qt.IsTrue)
	c.Assert(updatedUserTwoCommodity.OriginalPriceCurrency, qt.Equals, models.Currency(usd))

	userOneSettings, err := userOneRegistrySet.SettingsRegistry.Get(userOneCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(userOneSettings.MainCurrency, qt.IsNotNil)
	c.Assert(*userOneSettings.MainCurrency, qt.Equals, eur)

	userTwoSettings, err := userTwoRegistrySet.SettingsRegistry.Get(userTwoCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(userTwoSettings.MainCurrency, qt.IsNotNil)
	c.Assert(*userTwoSettings.MainCurrency, qt.Equals, usd)
}

func newSettingsTestEnv(t *testing.T) struct {
	router     http.Handler
	factorySet *registry.FactorySet
} {
	t.Helper()

	factorySet := memory.NewFactorySet()
	r := chi.NewRouter()
	r.Use(apiserver.RegistrySetMiddleware(factorySet))
	r.Route("/settings", apiserver.Settings())

	return struct {
		router     http.Handler
		factorySet *registry.FactorySet
	}{
		router:     r,
		factorySet: factorySet,
	}
}

func newUserRegistrySet(t *testing.T, factorySet *registry.FactorySet, userID, tenantID string) (context.Context, *registry.Set) {
	t.Helper()

	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			EntityID: models.EntityID{ID: userID},
		},
	})

	registrySet, err := factorySet.CreateUserRegistrySet(ctx)
	if err != nil {
		t.Fatalf("create user registry set: %v", err)
	}

	return ctx, registrySet
}

func createTestArea(t *testing.T, ctx context.Context, registrySet *registry.Set) *models.Area {
	t.Helper()

	location, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name:    "Test Location",
		Address: "Test Address",
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}

	area, err := registrySet.AreaRegistry.Create(ctx, models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	if err != nil {
		t.Fatalf("create area: %v", err)
	}

	return area
}

func createCommodity(t *testing.T, ctx context.Context, registrySet *registry.Set, commodity models.Commodity) *models.Commodity {
	t.Helper()

	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	if err != nil {
		t.Fatalf("create commodity: %v", err)
	}

	return createdCommodity
}

func performSettingsRequest(t *testing.T, router http.Handler, ctx context.Context, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader *bytes.Reader
	if body == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w
}

func decodeSettingsResponse(t *testing.T, response *httptest.ResponseRecorder) models.SettingsObject {
	t.Helper()

	var settings models.SettingsObject
	if err := json.Unmarshal(response.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	return settings
}
