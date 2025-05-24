package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type settingsAPI struct {
	registry registry.SettingsRegistry
}

// getSettings returns the current settings.
// @Summary Get current settings
// @Description get current settings
// @Tags settings
// @Accept  json
// @Produce  json
// @Success 200 {object} models.SettingsObject "OK"
// @Router /settings [get]
func (api *settingsAPI) getSettings(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get current settings
	settings, err := api.registry.Get(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")

	// Return the settings object
	if err := json.NewEncoder(w).Encode(settings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// updateSettings updates the entire settings object.
// @Summary Update settings
// @Description update entire settings object
// @Tags settings
// @Accept  json
// @Produce  json
// @Param   settings body models.SettingsObject true "Settings object"
// @Success 200 {object} models.SettingsObject "OK"
// @Router /settings [put]
func (api *settingsAPI) updateSettings(w http.ResponseWriter, r *http.Request) {
	// Decode the request body into a settings object
	var settings models.SettingsObject
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if main currency is being changed
	if settings.MainCurrency != nil {
		currentSettings, err := api.registry.Get(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If main currency is already set and the new value is different, return an error
		if currentSettings.MainCurrency != nil && *currentSettings.MainCurrency != "" && *settings.MainCurrency != *currentSettings.MainCurrency {
			http.Error(w, registry.ErrMainCurrencyAlreadySet.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	// Save the settings
	if err := api.registry.Save(r.Context(), settings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the updated settings
	updatedSettings, err := api.registry.Get(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")

	// Return the updated settings
	if err := json.NewEncoder(w).Encode(updatedSettings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// patchSetting updates a specific setting field.
// @Summary Patch setting
// @Description update a specific setting field
// @Tags settings
// @Accept  json
// @Produce  json
// @Param   field path string true "Setting field path (e.g., system.main_currency)"
// @Param   value body any true "Setting value"
// @Success 200 {object} models.SettingsObject "OK"
// @Router /settings/{field} [patch]
func (api *settingsAPI) patchSetting(w http.ResponseWriter, r *http.Request) {
	// Get the field path from the URL
	field := chi.URLParam(r, "field")
	if field == "" {
		http.Error(w, "Field path is required", http.StatusBadRequest)
		return
	}

	// Check if trying to update main currency
	if field == "system.main_currency" {
		currentSettings, err := api.registry.Get(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If main currency is already set, prevent changing it
		if currentSettings.MainCurrency != nil && *currentSettings.MainCurrency != "" {
			http.Error(w, registry.ErrMainCurrencyAlreadySet.Error(), http.StatusUnprocessableEntity)
			return
		}
	}

	// Decode the request body into a value
	var value any
	if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Patch the setting
	if err := api.registry.Patch(r.Context(), field, value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the updated settings
	updatedSettings, err := api.registry.Get(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")

	// Return the updated settings
	if err := json.NewEncoder(w).Encode(updatedSettings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Settings returns a handler for settings.
func Settings(settingsRegistry registry.SettingsRegistry) func(r chi.Router) {
	api := &settingsAPI{
		registry: settingsRegistry,
	}

	return func(r chi.Router) {
		r.Get("/", api.getSettings)           // GET /settings
		r.Put("/", api.updateSettings)        // PUT /settings
		r.Patch("/{field}", api.patchSetting) // PATCH /settings/{field}
	}
}
