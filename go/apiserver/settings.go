package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var (
	errRegistrySetNotFound       = errors.New("registry set not found in context")
	errInvalidMainCurrencyValue  = errors.New("invalid currency value")
	errPatchSettingValueRequired = errors.New("patch setting value is required")
	defaultSettingsRateProvider  = currency.NewDefaultRateProvider()
)

type settingsAPI struct {
}

// SettingsUpdateRequest documents the PUT /settings request body.
type SettingsUpdateRequest struct {
	// MainCurrency is the system.main_currency value accepted by PUT /settings.
	MainCurrency *string `json:"main_currency,omitempty"`
	// Theme is the uiconfig.theme value accepted by PUT /settings.
	Theme *string `json:"theme,omitempty"`
	// ShowDebugInfo is the uiconfig.show_debug_info value accepted by PUT /settings.
	ShowDebugInfo *bool `json:"show_debug_info,omitempty"`
	// DefaultDateFormat is the uiconfig.default_date_format value accepted by PUT /settings.
	DefaultDateFormat *string `json:"default_date_format,omitempty"`
	// ExchangeRate optionally overrides the conversion rate when the main currency changes.
	ExchangeRate *decimal.Decimal `json:"exchange_rate,omitempty"`
}

type legacySettingsUpdateRequest struct {
	models.SettingsObject
	ExchangeRate *decimal.Decimal `json:"exchange_rate,omitempty"`
}

// UnmarshalJSON accepts the documented snake_case PUT payload while preserving legacy compatibility.
func (r *SettingsUpdateRequest) UnmarshalJSON(data []byte) error {
	type requestAlias SettingsUpdateRequest

	var request requestAlias
	if err := json.Unmarshal(data, &request); err != nil {
		return err
	}

	*r = SettingsUpdateRequest(request)

	var legacy legacySettingsUpdateRequest
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}

	if r.MainCurrency == nil {
		r.MainCurrency = legacy.MainCurrency
	}
	if r.Theme == nil {
		r.Theme = legacy.Theme
	}
	if r.ShowDebugInfo == nil {
		r.ShowDebugInfo = legacy.ShowDebugInfo
	}
	if r.DefaultDateFormat == nil {
		r.DefaultDateFormat = legacy.DefaultDateFormat
	}
	if r.ExchangeRate == nil {
		r.ExchangeRate = legacy.ExchangeRate
	}

	return nil
}

func (r SettingsUpdateRequest) toSettingsObject() models.SettingsObject {
	return models.SettingsObject{
		MainCurrency:      r.MainCurrency,
		Theme:             r.Theme,
		ShowDebugInfo:     r.ShowDebugInfo,
		DefaultDateFormat: r.DefaultDateFormat,
	}
}

// PatchSettingRequest documents the object-form PATCH /settings/{field} request body.
// PATCH /settings/system.main_currency also accepts a raw JSON string body for backward compatibility.
type PatchSettingRequest struct {
	// Value is the setting value to apply and is required when using the object envelope.
	Value any `json:"value"`
	// ExchangeRate optionally overrides the conversion rate when the main currency changes.
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
	// Get user-aware settings registry from context
	registrySet, err := registrySetFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get user-aware settings registry
	settingsRegistry := registrySet.SettingsRegistry

	// Get current settings
	settings, err := settingsRegistry.Get(r.Context())
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
// @Param   settings body SettingsUpdateRequest true "Settings object with documented snake_case field names and optional exchange_rate when changing the main currency"
// @Success 200 {object} models.SettingsObject "OK"
// @Failure 400 {string} string "Bad Request"
// @Router /settings [put]
func (api *settingsAPI) updateSettings(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet, err := registrySetFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get user-aware settings registry
	settingsRegistry := registrySet.SettingsRegistry
	conversionService := currency.NewConversionService(registrySet.CommodityRegistry, defaultSettingsRateProvider)

	// Decode the request body into a settings object
	var req SettingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	settings := req.toSettingsObject()

	// Check if main currency is being changed
	if settings.MainCurrency != nil {
		err = api.handleMainCurrencyUpdate(r.Context(), settingsRegistry, conversionService, *settings.MainCurrency, req.ExchangeRate)
		if err != nil {
			http.Error(w, err.Error(), statusCodeForCurrencyMigrationError(err))
			return
		}
	}

	// Save the settings
	if err := settingsRegistry.Save(r.Context(), settings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the updated settings
	updatedSettings, err := settingsRegistry.Get(r.Context())
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
// @Description update a specific setting field. PATCH /settings/system.main_currency also accepts a raw JSON string body for backward compatibility.
// @Tags settings
// @Accept  json
// @Produce  json
// @Param   field path string true "Setting field path (e.g., system.main_currency)"
// @Param   value body PatchSettingRequest true "Setting value envelope with required value and optional exchange_rate. PATCH /settings/system.main_currency also accepts a raw JSON string body for backward compatibility."
// @Success 200 {object} models.SettingsObject "OK"
// @Failure 400 {string} string "Bad Request"
// @Router /settings/{field} [patch]
func (api *settingsAPI) patchSetting(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet, err := registrySetFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get user-aware settings registry
	settingsRegistry := registrySet.SettingsRegistry
	conversionService := currency.NewConversionService(registrySet.CommodityRegistry, defaultSettingsRateProvider)

	// Get the field path from the URL
	field := chi.URLParam(r, "field")
	if field == "" {
		http.Error(w, "Field path is required", http.StatusBadRequest)
		return
	}

	var rawValue json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawValue); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	value, exchangeRate, err := decodePatchSettingValue(rawValue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if trying to update main currency
	if field == string(models.SettingNameSystemMainCurrency) {
		err = api.handleMainCurrencyUpdate(r.Context(), settingsRegistry, conversionService, value, exchangeRate)
		if err != nil {
			http.Error(w, err.Error(), statusCodeForCurrencyMigrationError(err))
			return
		}
	}

	// Patch the setting
	if err := settingsRegistry.Patch(r.Context(), field, value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the updated settings
	updatedSettings, err := settingsRegistry.Get(r.Context())
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

func (api *settingsAPI) handleMainCurrencyUpdate(ctx context.Context, settingsRegistry registry.SettingsRegistry, conversionService *currency.ConversionService, value any, exchangeRate *decimal.Decimal) error {
	newCurrency, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w: %T", errInvalidMainCurrencyValue, value)
	}

	if !models.Currency(newCurrency).IsValid() {
		return fmt.Errorf("%w: %q", errInvalidMainCurrencyValue, newCurrency)
	}

	currentSettings, err := settingsRegistry.Get(ctx)
	if err != nil {
		return err
	}

	if currentSettings.MainCurrency == nil || *currentSettings.MainCurrency == "" || newCurrency == *currentSettings.MainCurrency {
		return nil
	}

	return conversionService.ConvertCommodityPricesWithRate(ctx, *currentSettings.MainCurrency, newCurrency, exchangeRate)
}

func decodePatchSettingValue(rawValue json.RawMessage) (any, *decimal.Decimal, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rawValue, &envelope); err == nil && hasPatchSettingEnvelopeShape(envelope) {
		var req PatchSettingRequest
		if err := json.Unmarshal(rawValue, &req); err != nil {
			return nil, nil, err
		}
		if req.Value == nil {
			return nil, nil, errPatchSettingValueRequired
		}

		return req.Value, req.ExchangeRate, nil
	}

	var value any
	if err := json.Unmarshal(rawValue, &value); err != nil {
		return nil, nil, err
	}

	return value, nil, nil
}

func hasPatchSettingEnvelopeShape(value map[string]json.RawMessage) bool {
	_, hasValue := value["value"]
	_, hasExchangeRate := value["exchange_rate"]

	return hasValue || hasExchangeRate
}

func registrySetFromContext(ctx context.Context) (*registry.Set, error) {
	registrySet := RegistrySetFromContext(ctx)
	if registrySet == nil {
		return nil, errRegistrySetNotFound
	}

	return registrySet, nil
}

func statusCodeForCurrencyMigrationError(err error) int {
	if errors.Is(err, errInvalidMainCurrencyValue) || errors.Is(err, currency.ErrExchangeRateRequired) || errors.Is(err, currency.ErrInvalidExchangeRate) {
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}

// Settings returns a handler for settings.
func Settings() func(r chi.Router) {
	api := &settingsAPI{}

	return func(r chi.Router) {
		r.Get("/", api.getSettings)           // GET /settings
		r.Put("/", api.updateSettings)        // PUT /settings
		r.Patch("/{field}", api.patchSetting) // PATCH /settings/{field}
	}
}
