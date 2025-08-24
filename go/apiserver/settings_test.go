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

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestSettingsAPI(t *testing.T) {
	c := qt.New(t)

	// Create a memory registry for testing
	registrySet := memory.NewRegistrySetWithUserID("test-user-id")
	c.Assert(registrySet, qt.IsNotNil)

	// Create a router with the settings endpoint
	r := chi.NewRouter()
	r.Route("/settings", apiserver.Settings(registrySet.SettingsRegistry))

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

func TestMainCurrencyRestriction(t *testing.T) {
	c := qt.New(t)

	// Create a memory registry for testing
	registrySet := memory.NewRegistrySet()
	c.Assert(registrySet, qt.IsNotNil)

	// Create a router with the settings endpoint
	r := chi.NewRouter()
	r.Route("/settings", apiserver.Settings(registrySet.SettingsRegistry))

	// First, set the main currency to USD
	currency := "USD"
	testSettings := models.SettingsObject{
		MainCurrency: &currency,
	}
	settingsJSON, err := json.Marshal(testSettings)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("PUT", "/settings", bytes.NewReader(settingsJSON))
	req = req.WithContext(appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// Now try to change the main currency to EUR using PUT
	newCurrency := "EUR"
	testSettings.MainCurrency = &newCurrency
	settingsJSON, err = json.Marshal(testSettings)
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

	// Should get an error
	c.Assert(w.Code, qt.Equals, http.StatusUnprocessableEntity)

	// Try to change the main currency to EUR using PATCH
	currencyJSON, err := json.Marshal(newCurrency)
	c.Assert(err, qt.IsNil)

	req = httptest.NewRequest("PATCH", "/settings/system.main_currency", bytes.NewReader(currencyJSON))
	req = req.WithContext(appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should get an error
	c.Assert(w.Code, qt.Equals, http.StatusUnprocessableEntity)

	// Verify the main currency is still USD
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

	var retrievedSettings models.SettingsObject
	err = json.Unmarshal(w.Body.Bytes(), &retrievedSettings)
	c.Assert(err, qt.IsNil)
	c.Assert(*retrievedSettings.MainCurrency, qt.Equals, currency) // Should still be USD
}
