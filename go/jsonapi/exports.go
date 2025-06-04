package jsonapi

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

type ExportResponse struct {
	HTTPStatusCode int                `json:"-"` // http response status code
	Data           *ExportResponseData `json:"data"`
}

// ExportResponseData is an object that holds export information.
type ExportResponseData struct {
	ID         string        `json:"id"`
	Type       string        `json:"type" example:"exports" enums:"exports"`
	Attributes models.Export `json:"attributes"`
}

func NewExportResponse(export *models.Export) *ExportResponse {
	return &ExportResponse{
		Data: &ExportResponseData{
			ID:         export.ID,
			Type:       "exports",
			Attributes: *export,
		},
	}
}

func (rd *ExportResponse) WithStatusCode(statusCode int) *ExportResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *ExportResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

// ExportsMeta is a meta information for ExportsResponse.
type ExportsMeta struct {
	Exports int `json:"exports" example:"1" format:"int64"`
}

type ExportsResponse struct {
	HTTPStatusCode int                   `json:"-"` // http response status code
	Data           []*ExportResponseData `json:"data"`
	Meta           *ExportsMeta          `json:"meta"`
}

func NewExportsResponse(exports []*models.Export, total int) *ExportsResponse {
	data := make([]*ExportResponseData, len(exports))
	for i, export := range exports {
		data[i] = &ExportResponseData{
			ID:         export.ID,
			Type:       "exports",
			Attributes: *export,
		}
	}

	return &ExportsResponse{
		Data: data,
		Meta: &ExportsMeta{
			Exports: total,
		},
	}
}

func (rd *ExportsResponse) WithStatusCode(statusCode int) *ExportsResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *ExportsResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

// ExportCreateRequestData is request data for creating an export.
type ExportCreateRequestData struct {
	Type       string        `json:"type" example:"exports" enums:"exports"`
	Attributes models.Export `json:"attributes"`
}

func (d *ExportCreateRequestData) ToModel() models.Export {
	return d.Attributes
}

type ExportCreateRequest struct {
	Data *ExportCreateRequestData `json:"data"`
}

func (*ExportCreateRequest) Bind(_r *http.Request) error {
	return nil
}

func (r *ExportCreateRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&r.Data, validation.Required),
	)

	if err := validation.ValidateStructWithContext(ctx, r, fields...); err != nil {
		return err
	}

	if r.Data != nil {
		return r.Data.Attributes.ValidateWithContext(ctx)
	}

	return nil
}

// ExportUpdateRequestData is request data for updating an export.
type ExportUpdateRequestData struct {
	Type       string        `json:"type" example:"exports" enums:"exports"`
	Attributes models.Export `json:"attributes"`
}

func (d *ExportUpdateRequestData) ToModel() models.Export {
	return d.Attributes
}

type ExportUpdateRequest struct {
	Data *ExportUpdateRequestData `json:"data"`
}

func (*ExportUpdateRequest) Bind(_r *http.Request) error {
	return nil
}

func (r *ExportUpdateRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&r.Data, validation.Required),
	)

	if err := validation.ValidateStructWithContext(ctx, r, fields...); err != nil {
		return err
	}

	if r.Data != nil {
		return r.Data.Attributes.ValidateWithContext(ctx)
	}

	return nil
}
