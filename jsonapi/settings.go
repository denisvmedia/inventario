package jsonapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

// SettingResponse is an object that holds setting information.
type SettingResponse struct {
	HTTPStatusCode int                   `json:"-"` // http response status code
	Data           *SettingResponseData `json:"data"`
}

// SettingResponseData is an object that holds setting data information.
type SettingResponseData struct {
	ID         string         `json:"id"`
	Type       string         `json:"type" example:"settings" enums:"settings"`
	Attributes *models.Setting `json:"attributes"`
}

// NewSettingResponse creates a new SettingResponse instance.
func NewSettingResponse(setting *models.Setting) *SettingResponse {
	return &SettingResponse{
		HTTPStatusCode: http.StatusOK,
		Data: &SettingResponseData{
			ID:         setting.ID,
			Type:       "settings",
			Attributes: setting,
		},
	}
}

// NewSettingResponseWithValue creates a new SettingResponse instance with a parsed value.
func NewSettingResponseWithValue(id string, value any) *SettingResponse {
	// Marshal the value to JSON
	valueJSON, _ := json.Marshal(value)

	setting := &models.Setting{
		ID:    id,
		Value: valueJSON,
	}

	return NewSettingResponse(setting)
}

// Render implements the render.Renderer interface for SettingResponse.
func (sr *SettingResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, sr.HTTPStatusCode)
	return nil
}

// SettingsListResponse is an object that holds settings list information.
type SettingsListResponse struct {
	HTTPStatusCode int                     `json:"-"` // http response status code
	Data           []*SettingResponseData `json:"data"`
}

// NewSettingsListResponse creates a new SettingsListResponse instance.
func NewSettingsListResponse(settings []*models.Setting) *SettingsListResponse {
	data := make([]*SettingResponseData, len(settings))
	for i, setting := range settings {
		data[i] = &SettingResponseData{
			ID:         setting.ID,
			Type:       "settings",
			Attributes: setting,
		}
	}

	return &SettingsListResponse{
		HTTPStatusCode: http.StatusOK,
		Data:           data,
	}
}

// Render implements the render.Renderer interface for SettingsListResponse.
func (slr *SettingsListResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, slr.HTTPStatusCode)
	return nil
}

// SettingRequest is an object that holds setting request information.
type SettingRequest struct {
	Data *SettingData `json:"data"`
}

// SettingData is an object that holds setting data information.
type SettingData struct {
	ID         string         `json:"id,omitempty"`
	Type       string         `json:"type" example:"settings" enums:"settings"`
	Attributes *models.Setting `json:"attributes"`
}

// Validate validates the SettingData.
func (sd *SettingData) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&sd.Type, validation.Required, validation.In("settings")),
		validation.Field(&sd.Attributes, validation.Required),
	)
	return validation.ValidateStruct(sd, fields...)
}

// Bind implements the render.Binder interface for SettingRequest.
func (sr *SettingRequest) Bind(r *http.Request) error {
	return nil
}
