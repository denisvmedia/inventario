package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type settingsAPI struct {
	registry          registry.SettingsRegistry
	conversionService *currency.ConversionService
}

// SettingsUpdateRequest represents a request to update settings with optional exchange rate
type SettingsUpdateRequest struct {
	models.SettingsObject
	ExchangeRate *decimal.Decimal `json:"exchange_rate,omitempty"`
}

// PatchSettingRequest represents a request to patch a setting with optional exchange rate
type PatchSettingRequest struct {
	Value        any              `json:"value"`
	ExchangeRate *decimal.Decimal `json:"exchange_rate,omitempty"`
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
// @Param   settings body any true "Settings object or SettingsUpdateRequest with optional exchange rate"
// @Success 200 {object} models.SettingsObject "OK"
// @Router /settings [put]
func (api *settingsAPI) updateSettings(w http.ResponseWriter, r *http.Request) {
	// Try to decode as SettingsUpdateRequest first, then fallback to direct SettingsObject
	var rawData json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Try to decode as SettingsUpdateRequest
	var req SettingsUpdateRequest
	var settings models.SettingsObject
	var exchangeRate *decimal.Decimal

	if err := json.Unmarshal(rawData, &req); err == nil && (req.MainCurrency != nil || req.Theme != nil || req.ShowDebugInfo != nil || req.DefaultDateFormat != nil || req.ExchangeRate != nil) {
		// Successfully decoded as SettingsUpdateRequest
		settings = req.SettingsObject
		exchangeRate = req.ExchangeRate
	} else {
		// Fallback to direct SettingsObject (backward compatibility)
		if err := json.Unmarshal(rawData, &settings); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Check if main currency is being changed
	if settings.MainCurrency != nil {
		currentSettings, err := api.registry.Get(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If main currency is already set and the new value is different, perform conversion
		if currentSettings.MainCurrency != nil && *currentSettings.MainCurrency != "" && *settings.MainCurrency != *currentSettings.MainCurrency {
			// Convert all commodity prices from old currency to new currency
			err = api.conversionService.ConvertCommodityPricesWithRate(r.Context(), *currentSettings.MainCurrency, *settings.MainCurrency, exchangeRate)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
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
// @Param   request body any true "Setting value or PatchSettingRequest with optional exchange rate"
// @Success 200 {object} models.SettingsObject "OK"
// @Router /settings/{field} [patch]
func (api *settingsAPI) patchSetting(w http.ResponseWriter, r *http.Request) {
	// Get the field path from the URL
	field := chi.URLParam(r, "field")
	if field == "" {
		http.Error(w, "Field path is required", http.StatusBadRequest)
		return
	}

	// Try to decode as PatchSettingRequest first, then fallback to direct value
	var rawData json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Try to decode as PatchSettingRequest
	var req PatchSettingRequest
	var value any
	var exchangeRate *decimal.Decimal

	if err := json.Unmarshal(rawData, &req); err == nil && req.Value != nil {
		// Successfully decoded as PatchSettingRequest
		value = req.Value
		exchangeRate = req.ExchangeRate
	} else {
		// Fallback to direct value (backward compatibility)
		if err := json.Unmarshal(rawData, &value); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Check if trying to update main currency
	if field == "system.main_currency" {
		err := api.handleMainCurrencyUpdate(r.Context(), value, exchangeRate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

// handleMainCurrencyUpdate handles the logic for updating main currency with conversion
func (api *settingsAPI) handleMainCurrencyUpdate(ctx context.Context, value any, exchangeRate *decimal.Decimal) error {
	currentSettings, err := api.registry.Get(ctx)
	if err != nil {
		return err
	}

	// If main currency is already set and the new value is different, perform conversion
	if currentSettings.MainCurrency != nil && *currentSettings.MainCurrency != "" {
		newCurrency, ok := value.(string)
		if !ok {
			return fmt.Errorf("invalid currency value")
		}

		if newCurrency != *currentSettings.MainCurrency {
			// Convert all commodity prices from old currency to new currency
			err = api.conversionService.ConvertCommodityPricesWithRate(ctx, *currentSettings.MainCurrency, newCurrency, exchangeRate)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Settings returns a handler for settings.
func Settings(settingsRegistry registry.SettingsRegistry, conversionService *currency.ConversionService) func(r chi.Router) {
	api := &settingsAPI{
		registry:          settingsRegistry,
		conversionService: conversionService,
	}

	return func(r chi.Router) {
		r.Get("/", api.getSettings)           // GET /settings
		r.Put("/", api.updateSettings)        // PUT /settings
		r.Patch("/{field}", api.patchSetting) // PATCH /settings/{field}
	}
}
