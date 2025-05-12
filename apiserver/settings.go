package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type settingsAPI struct {
	settingsRegistry registry.SettingsRegistry
}

// listSettings lists all settings.
// @Summary List all settings
// @Description get settings
// @Tags settings
// @Accept  json-api
// @Produce  json-api
// @Success 200 {array} jsonapi.SettingResponse "OK"
// @Router /settings [get].
func (api *settingsAPI) listSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := api.settingsRegistry.List()
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	resp := jsonapi.NewSettingsListResponse(settings)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getSetting gets a setting by ID.
// @Summary Get a setting
// @Description get setting by ID
// @Tags settings
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Setting ID"
// @Success 200 {object} jsonapi.SettingResponse "OK"
// @Router /settings/{name} [get].
func (api *settingsAPI) getSetting(w http.ResponseWriter, r *http.Request) {
	settingName := chi.URLParam(r, "name")
	if settingName == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	var result any
	var err error

	switch settingName {
	case "ui_config":
		result, err = api.settingsRegistry.GetUIConfig()
	case "system_config":
		result, err = api.settingsRegistry.GetSystemConfig()
	default:
		err = registry.ErrNotFound
	}

	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewSettingResponseWithValue(settingName, result)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// setSetting updates a setting.
// @Summary Update a setting
// @Description Update by setting data
// @Tags settings
// @Accept json-api
// @Produce json-api
// @Param id path string true "Setting ID"
// @Param setting body jsonapi.SettingRequest true "Setting object"
// @Success 200 {object} jsonapi.SettingResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Setting not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /settings/{name} [put].
func (api *settingsAPI) setSetting(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.SettingRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	err := validation.Validate(input.Data)
	if err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	setting := input.Data.Attributes
	setting.Name = name

	var result any

	switch name {
	case "ui_config":
		var config models.UIConfig
		if err := json.Unmarshal(setting.Value, &config); err != nil {
			unprocessableEntityError(w, r, err)
			return
		}
		err = api.settingsRegistry.SetUIConfig(&config)
		result = &config
	case "system_config":
		var config models.SystemConfig
		if err := json.Unmarshal(setting.Value, &config); err != nil {
			unprocessableEntityError(w, r, err)
			return
		}
		err = api.settingsRegistry.SetSystemConfig(&config)
		result = &config
	default:
		err = registry.ErrNotFound
	}

	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewSettingResponseWithValue(name, result)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteSetting deletes a setting by ID.
// @Summary Delete a setting
// @Description Delete by setting ID
// @Tags settings
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Setting ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Setting not found"
// @Router /settings/{name} [delete].
func (api *settingsAPI) deleteSetting(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	var err error

	switch name {
	case "ui_config":
		err = api.settingsRegistry.RemoveUIConfig()
	case "system_config":
		err = api.settingsRegistry.RemoveSystemConfig()
	default:
		err = registry.ErrNotFound
	}

	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Settings returns a handler for settings.
func Settings(settingsRegistry registry.SettingsRegistry) func(r chi.Router) {
	api := &settingsAPI{
		settingsRegistry: settingsRegistry,
	}

	return func(r chi.Router) {
		r.Get("/", api.listSettings)           // GET /settings
		r.Get("/{name}", api.getSetting)       // GET /settings/:name
		r.Put("/{name}", api.setSetting)       // PUT /settings/:name
		r.Delete("/{name}", api.deleteSetting) // DELETE /settings/:name
	}
}
