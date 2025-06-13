package jsonapi

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type ExportResponse struct {
	HTTPStatusCode int                 `json:"-"` // http response status code
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
	Type       string         `json:"type" example:"exports" enums:"exports"`
	Attributes *models.Export `json:"attributes"`
}

func (cd *ExportCreateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cd.Type, validation.Required, validation.In("exports")),
		validation.Field(&cd.Attributes, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, cd, fields...)
}

type ExportCreateRequest struct {
	Data *ExportCreateRequestData `json:"data"`
}

var _ render.Binder = (*ExportCreateRequest)(nil)

func (cr *ExportCreateRequest) Bind(r *http.Request) error {
	err := cr.ValidateWithContext(r.Context())
	if err != nil {
		return err
	}

	return nil
}

func (cr *ExportCreateRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cr.Data, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, cr, fields...)
}

// ImportExportAttributes holds the attributes for importing an export.
type ImportExportAttributes struct {
	Description    string `json:"description"`
	SourceFilePath string `json:"source_file_path"`
}

func (a *ImportExportAttributes) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&a.Description, validation.Required, validation.Length(1, 500)),
		validation.Field(&a.SourceFilePath, validation.Required, validation.Length(1, 1000)),
	)
	return validation.ValidateStructWithContext(ctx, a, fields...)
}

// ImportExportRequestData is request data for importing an export.
type ImportExportRequestData struct {
	Type       string                   `json:"type" example:"exports" enums:"exports"`
	Attributes *ImportExportAttributes `json:"attributes"`
}

func (cd *ImportExportRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cd.Type, validation.Required, validation.In("exports")),
		validation.Field(&cd.Attributes, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, cd, fields...)
}

type ImportExportRequest struct {
	Data *ImportExportRequestData `json:"data"`
}

var _ render.Binder = (*ImportExportRequest)(nil)

func (cr *ImportExportRequest) Bind(r *http.Request) error {
	if cr.Data == nil {
		return errkit.WithMessage(nil, "missing required data field")
	}

	if cr.Data.Type != "exports" {
		return errkit.WithMessage(nil, "invalid type, expected 'exports'")
	}

	if cr.Data.Attributes == nil {
		return errkit.WithMessage(nil, "missing required attributes field")
	}

	// Validate the data structure
	if err := cr.Data.ValidateWithContext(r.Context()); err != nil {
		return err
	}

	// Validate the attributes
	return cr.Data.Attributes.ValidateWithContext(r.Context())
}

func (cr *ImportExportRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cr.Data, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, cr, fields...)
}
