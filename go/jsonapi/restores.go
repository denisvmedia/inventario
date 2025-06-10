package jsonapi

import (
	"errors"
	"net/http"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/restore"
)

type RestoreResponse struct {
	HTTPStatusCode int                  `json:"-"` // http response status code
	Data           *RestoreResponseData `json:"data"`
}

// RestoreResponseData is an object that holds restore information.
type RestoreResponseData struct {
	ID         string        `json:"id"`
	Type       string        `json:"type" example:"restores" enums:"restores"`
	Attributes models.Import `json:"attributes"`
}

func (rd *RestoreResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// Pre-processing before a response is marshalled and sent across the wire
	rd.HTTPStatusCode = 200
	return nil
}

func NewRestoreResponse(restore *models.Import) *RestoreResponse {
	resp := &RestoreResponse{Data: &RestoreResponseData{
		ID:         restore.ID,
		Type:       "restores",
		Attributes: *restore,
	}}

	return resp
}

func (rd *RestoreResponse) WithStatusCode(statusCode int) *RestoreResponse {
	rd.HTTPStatusCode = statusCode
	return rd
}

type RestoresResponse struct {
	HTTPStatusCode int                    `json:"-"` // http response status code
	Data           []*RestoreResponseData `json:"data"`
}

func (rd *RestoresResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// Pre-processing before a response is marshalled and sent across the wire
	rd.HTTPStatusCode = 200
	return nil
}

func NewRestoresResponse(restores []*models.Import) *RestoresResponse {
	list := make([]*RestoreResponseData, len(restores))
	for i, restore := range restores {
		list[i] = &RestoreResponseData{
			ID:         restore.ID,
			Type:       "restores",
			Attributes: *restore,
		}
	}

	resp := &RestoresResponse{Data: list}
	return resp
}

type RestoreCreateRequest struct {
	Data    *RestoreCreateRequestData `json:"data"`
	Options restore.RestoreOptions    `json:"options"`
}

// RestoreCreateRequestData is an object that holds restore creation information.
type RestoreCreateRequestData struct {
	Type       string         `json:"type" example:"restores" enums:"restores"`
	Attributes *models.Import `json:"attributes"`
}

func (a *RestoreCreateRequest) Bind(r *http.Request) error {
	// a.Data is nil if no Data fields are sent in the request. Return an
	// error to avoid a runtime panic.
	if a.Data == nil {
		return errors.New("missing required Data fields")
	}

	// a.Data.Attributes is nil if no Attributes fields are sent in the request. Return an
	// error to avoid a runtime panic.
	if a.Data.Attributes == nil {
		return errors.New("missing required Attributes fields")
	}

	// Validate the restore options
	if err := a.validateOptions(); err != nil {
		return errkit.Wrap(err, "invalid restore options")
	}

	// Validate the restore attributes
	if err := a.Data.Attributes.ValidateWithContext(r.Context()); err != nil {
		return errkit.Wrap(err, "validation failed")
	}

	return nil
}

func (a *RestoreCreateRequest) validateOptions() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.Options.Strategy, validation.Required, validation.In(
			restore.RestoreStrategyFullReplace,
			restore.RestoreStrategyMergeAdd,
			restore.RestoreStrategyMergeUpdate,
		)),
	)

	return validation.ValidateStruct(&a.Options, fields...)
}

type RestoreUpdateRequest struct {
	Data *RestoreUpdateRequestData `json:"data"`
}

// RestoreUpdateRequestData is an object that holds restore update information.
type RestoreUpdateRequestData struct {
	Type       string         `json:"type" example:"restores" enums:"restores"`
	Attributes *models.Import `json:"attributes"`
}

func (a *RestoreUpdateRequest) Bind(r *http.Request) error {
	// a.Data is nil if no Data fields are sent in the request. Return an
	// error to avoid a runtime panic.
	if a.Data == nil {
		return errors.New("missing required Data fields")
	}

	// a.Data.Attributes is nil if no Attributes fields are sent in the request. Return an
	// error to avoid a runtime panic.
	if a.Data.Attributes == nil {
		return errors.New("missing required Attributes fields")
	}

	if err := a.Data.Attributes.ValidateWithContext(r.Context()); err != nil {
		return errkit.Wrap(err, "validation failed")
	}

	return nil
}

// RestorePreviewRequest represents a request to preview a restore operation
type RestorePreviewRequest struct {
	Data    *RestorePreviewRequestData `json:"data"`
	Options restore.RestoreOptions     `json:"options"`
}

// RestorePreviewRequestData is an object that holds restore preview information.
type RestorePreviewRequestData struct {
	Type       string `json:"type" example:"restore-preview" enums:"restore-preview"`
	Attributes struct {
		SourceFilePath string `json:"source_file_path"`
	} `json:"attributes"`
}

func (a *RestorePreviewRequest) Bind(r *http.Request) error {
	// a.Data is nil if no Data fields are sent in the request. Return an
	// error to avoid a runtime panic.
	if a.Data == nil {
		return errors.New("missing required Data fields")
	}

	// Validate the restore options
	if err := a.validateOptions(); err != nil {
		return errkit.Wrap(err, "invalid restore options")
	}

	// Validate required fields
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&a.Data.Attributes.SourceFilePath, validation.Required),
	)

	return validation.ValidateStructWithContext(r.Context(), &a.Data.Attributes, fields...)
}

func (a *RestorePreviewRequest) validateOptions() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.Options.Strategy, validation.Required, validation.In(
			restore.RestoreStrategyFullReplace,
			restore.RestoreStrategyMergeAdd,
			restore.RestoreStrategyMergeUpdate,
		)),
	)

	return validation.ValidateStruct(&a.Options, fields...)
}

// RestorePreviewResponse represents the response for a restore preview
type RestorePreviewResponse struct {
	HTTPStatusCode int                         `json:"-"` // http response status code
	Data           *RestorePreviewResponseData `json:"data"`
}

// RestorePreviewResponseData is an object that holds restore preview information.
type RestorePreviewResponseData struct {
	Type       string                `json:"type" example:"restore-preview" enums:"restore-preview"`
	Attributes *restore.RestoreStats `json:"attributes"`
}

func (rd *RestorePreviewResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// Pre-processing before a response is marshalled and sent across the wire
	rd.HTTPStatusCode = 200
	return nil
}

func NewRestorePreviewResponse(stats *restore.RestoreStats) *RestorePreviewResponse {
	resp := &RestorePreviewResponse{Data: &RestorePreviewResponseData{
		Type:       "restore-preview",
		Attributes: stats,
	}}

	return resp
}

func (rd *RestorePreviewResponse) WithStatusCode(statusCode int) *RestorePreviewResponse {
	rd.HTTPStatusCode = statusCode
	return rd
}
