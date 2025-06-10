package jsonapi

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type ImportResponse struct {
	HTTPStatusCode int                 `json:"-"` // http response status code
	Data           *ImportResponseData `json:"data"`
}

// ImportResponseData is an object that holds import information.
type ImportResponseData struct {
	ID         string        `json:"id"`
	Type       string        `json:"type" example:"imports" enums:"imports"`
	Attributes models.Import `json:"attributes"`
}

func NewImportResponse(import_ *models.Import) *ImportResponse {
	return &ImportResponse{
		Data: &ImportResponseData{
			ID:         import_.ID,
			Type:       "imports",
			Attributes: *import_,
		},
	}
}

func (rd *ImportResponse) WithStatusCode(statusCode int) *ImportResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *ImportResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

// ImportsMeta is a meta information for ImportsResponse.
type ImportsMeta struct {
	Imports int `json:"imports" example:"1" format:"int64"`
}

type ImportsResponse struct {
	HTTPStatusCode int                   `json:"-"` // http response status code
	Data           []*ImportResponseData `json:"data"`
	Meta           *ImportsMeta          `json:"meta"`
}

func NewImportsResponse(imports []*models.Import) *ImportsResponse {
	data := make([]*ImportResponseData, len(imports))
	for i, import_ := range imports {
		data[i] = &ImportResponseData{
			ID:         import_.ID,
			Type:       "imports",
			Attributes: *import_,
		}
	}

	return &ImportsResponse{
		Data: data,
		Meta: &ImportsMeta{
			Imports: len(imports),
		},
	}
}

func (rd *ImportsResponse) WithStatusCode(statusCode int) *ImportsResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *ImportsResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

// ImportCreateRequestData is request data for creating an import.
type ImportCreateRequestData struct {
	Type       string         `json:"type" example:"imports" enums:"imports"`
	Attributes *models.Import `json:"attributes"`
}

func (cd *ImportCreateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cd.Type, validation.Required, validation.In("imports")),
		validation.Field(&cd.Attributes, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, cd, fields...)
}

type ImportCreateRequest struct {
	Data *ImportCreateRequestData `json:"data"`
}

var _ render.Binder = (*ImportCreateRequest)(nil)

func (cr *ImportCreateRequest) Bind(r *http.Request) error {
	if cr.Data == nil {
		return errkit.WithMessage(nil, "missing required data field")
	}

	if err := cr.Data.ValidateWithContext(r.Context()); err != nil {
		return err
	}

	if cr.Data.Attributes == nil {
		return errkit.WithMessage(nil, "missing required attributes field")
	}

	return cr.Data.Attributes.ValidateWithContext(r.Context())
}

func (cr *ImportCreateRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cr.Data, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, cr, fields...)
}
