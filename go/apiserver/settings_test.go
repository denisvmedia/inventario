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

	// Create a memory factory set for testing
	factorySet := memory.NewFactorySet()
	c.Assert(factorySet, qt.IsNotNil)

	// Create a router with the settings endpoint and registry middleware
	r := chi.NewRouter()
	r.Use(apiserver.RegistrySetMiddleware(factorySet))
	r.Route("/settings", apiserver.Settings())

	userCtx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})

	// Test GET /settings (empty settings)
	req := httptest.NewRequest("GET", "/settings", nil)
	req = req.WithContext(userCtx)
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
	req = req.WithContext(userCtx)
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
	req = req.WithContext(userCtx)
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
	req = req.WithContext(userCtx)
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
	req = req.WithContext(userCtx)
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

// TestSettingsAPI_PatchUnknownField_ReturnsBadRequest pins the contract
// that a stale client calling PATCH /settings/system.main_currency (or any
// other removed/unknown field) receives 400 — not 500. The old endpoint
// returned 2xx, so returning 500 here would look like a server bug rather
// than a client-side obsolescence signal.
func TestSettingsAPI_PatchUnknownField_ReturnsBadRequest(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	c.Assert(factorySet, qt.IsNotNil)

	r := chi.NewRouter()
	r.Use(apiserver.RegistrySetMiddleware(factorySet))
	r.Route("/settings", apiserver.Settings())

	userCtx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})

	body, err := json.Marshal("EUR")
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest("PATCH", "/settings/system.main_currency", bytes.NewReader(body))
	req = req.WithContext(userCtx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusBadRequest, qt.Commentf("body: %s", w.Body.String()))
}
