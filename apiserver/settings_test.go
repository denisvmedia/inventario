package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func newSettingsRegistry() registry.SettingsRegistry {
	var settingsRegistry = memory.NewSettingsRegistry()

	// Add a test UI config
	must.Must0(settingsRegistry.SetUIConfig(&models.UIConfig{
		Theme:             "light",
		ShowDebugInfo:     false,
		DefaultPageSize:   20,
		DefaultDateFormat: "YYYY-MM-DD",
	}))

	// Add a test system config
	must.Must0(settingsRegistry.SetSystemConfig(&models.SystemConfig{
		UploadSizeLimit: 10485760, // 10MB
		LogLevel:        "info",
		BackupEnabled:   false,
		BackupInterval:  "24h",
		BackupLocation:  "",
		MainCurrency:    "USD",
	}))

	return settingsRegistry
}

func newParamsWithSettings() apiserver.Params {
	params := newParams()
	params.RegistrySet.SettingsRegistry = newSettingsRegistry()
	return params
}

func TestSettingsList(t *testing.T) {
	c := qt.New(t)

	params := newParamsWithSettings()

	req, err := http.NewRequest("GET", "/api/v1/settings", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	// Check that we have at least 2 settings (ui_config and system_config)
	c.Assert(body, checkers.JSONPathExists("$.data[0]"))
	c.Assert(body, checkers.JSONPathExists("$.data[1]"))
}

func TestSettingsGet(t *testing.T) {
	c := qt.New(t)

	params := newParamsWithSettings()

	req, err := http.NewRequest("GET", "/api/v1/settings/ui_config", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data.id"), "ui_config")
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "settings")
}

func TestSettingsGetSystemConfig(t *testing.T) {
	c := qt.New(t)

	params := newParamsWithSettings()

	req, err := http.NewRequest("GET", "/api/v1/settings/system_config", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data.id"), "system_config")
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "settings")

	// Parse the value to check the main currency
	var setting struct {
		Data struct {
			Attributes struct {
				Value json.RawMessage `json:"value"`
			} `json:"attributes"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &setting)
	c.Assert(err, qt.IsNil)

	var systemConfig models.SystemConfig
	err = json.Unmarshal(setting.Data.Attributes.Value, &systemConfig)
	c.Assert(err, qt.IsNil)
	c.Assert(systemConfig.MainCurrency, qt.Equals, "USD")
}

func TestSettingsCreate(t *testing.T) {
	c := qt.New(t)

	params := newParamsWithSettings()

	// Create a new system config with a different main currency
	systemConfig := models.SystemConfig{
		UploadSizeLimit: 20971520, // 20MB
		LogLevel:        "debug",
		BackupEnabled:   true,
		BackupInterval:  "12h",
		BackupLocation:  "/backup",
		MainCurrency:    "EUR",
	}

	// Marshal the system config
	configBytes, err := json.Marshal(systemConfig)
	c.Assert(err, qt.IsNil)

	// Create the request payload
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "settings",
			"id":   "system_config",
			"attributes": map[string]interface{}{
				"value": configBytes,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/api/v1/settings", bytes.NewBuffer(payloadBytes))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/vnd.api+json")

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data.id"), "system_config")
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "settings")

	// Now get the system config to verify it was updated
	req, err = http.NewRequest("GET", "/api/v1/settings/system_config", nil)
	c.Assert(err, qt.IsNil)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body = rr.Body.Bytes()

	// Parse the value to check the main currency
	var setting struct {
		Data struct {
			Attributes struct {
				Value json.RawMessage `json:"value"`
			} `json:"attributes"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &setting)
	c.Assert(err, qt.IsNil)

	var retrievedConfig models.SystemConfig
	err = json.Unmarshal(setting.Data.Attributes.Value, &retrievedConfig)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedConfig.MainCurrency, qt.Equals, "EUR")
}

func TestSettingsUpdate(t *testing.T) {
	c := qt.New(t)

	params := newParamsWithSettings()

	// Create a new UI config
	uiConfig := models.UIConfig{
		Theme:             "dark",
		ShowDebugInfo:     true,
		DefaultPageSize:   50,
		DefaultDateFormat: "DD/MM/YYYY",
	}

	// Marshal the UI config
	configBytes, err := json.Marshal(uiConfig)
	c.Assert(err, qt.IsNil)

	// Create the request payload
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "settings",
			"id":   "ui_config",
			"attributes": map[string]interface{}{
				"value": configBytes,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("PUT", "/api/v1/settings/ui_config", bytes.NewBuffer(payloadBytes))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/vnd.api+json")

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data.id"), "ui_config")
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "settings")

	// Now get the UI config to verify it was updated
	req, err = http.NewRequest("GET", "/api/v1/settings/ui_config", nil)
	c.Assert(err, qt.IsNil)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body = rr.Body.Bytes()

	// Parse the value to check the theme
	var setting struct {
		Data struct {
			Attributes struct {
				Value json.RawMessage `json:"value"`
			} `json:"attributes"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &setting)
	c.Assert(err, qt.IsNil)

	var retrievedConfig models.UIConfig
	err = json.Unmarshal(setting.Data.Attributes.Value, &retrievedConfig)
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedConfig.Theme, qt.Equals, "dark")
	c.Assert(retrievedConfig.DefaultDateFormat, qt.Equals, "DD/MM/YYYY")
}

func TestSettingsDelete(t *testing.T) {
	c := qt.New(t)

	params := newParamsWithSettings()

	req, err := http.NewRequest("DELETE", "/api/v1/settings/ui_config", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	// Now try to get the UI config, should fail
	req, err = http.NewRequest("GET", "/api/v1/settings/ui_config", nil)
	c.Assert(err, qt.IsNil)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}
