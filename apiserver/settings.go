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
// @Router /settings/{id} [get].
func (api *settingsAPI) getSetting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	var result any
	var err error

	switch id {
	case "ui_config":
		result, err = api.settingsRegistry.GetUIConfig()
	case "system_config":
		result, err = api.settingsRegistry.GetSystemConfig()
	default:
		// For any other ID, try to get the setting directly
		setting, err := api.settingsRegistry.Get(id)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
		resp := jsonapi.NewSettingResponse(setting)
		if err := render.Render(w, r, resp); err != nil {
			internalServerError(w, r, err)
		}
		return
	}

	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewSettingResponseWithValue(id, result)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createSetting creates a new setting.
// @Summary Create a new setting
// @Description add by setting data
// @Tags settings
// @Accept json-api
// @Produce json-api
// @Param setting body jsonapi.SettingRequest true "Setting object"
// @Success 201 {object} jsonapi.SettingResponse "Setting created"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /settings [post].
func (api *settingsAPI) createSetting(w http.ResponseWriter, r *http.Request) {
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
	id := input.Data.ID

	var result any

	switch id {
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
		// For any other ID, create the setting directly
		setting.ID = id
		result, err = api.settingsRegistry.Create(*setting)
	}

	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewSettingResponseWithValue(id, result)
	resp.HTTPStatusCode = http.StatusCreated
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// updateSetting updates a setting.
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
// @Router /settings/{id} [put].
func (api *settingsAPI) updateSetting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
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
	setting.ID = id

	var result any

	switch id {
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
		// For any other ID, update the setting directly
		result, err = api.settingsRegistry.Update(*setting)
	}

	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewSettingResponseWithValue(id, result)
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
// @Router /settings/{id} [delete].
func (api *settingsAPI) deleteSetting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	var err error

	switch id {
	case "ui_config":
		err = api.settingsRegistry.RemoveUIConfig()
	case "system_config":
		err = api.settingsRegistry.RemoveSystemConfig()
	default:
		// For any other ID, delete the setting directly
		err = api.settingsRegistry.Delete(id)
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
		r.Get("/", api.listSettings)       // GET /settings
		r.Post("/", api.createSetting)     // POST /settings
		r.Get("/{id}", api.getSetting)     // GET /settings/tls
		r.Put("/{id}", api.updateSetting)  // PUT /settings/tls
		r.Delete("/{id}", api.deleteSetting) // DELETE /settings/tls
	}
}
