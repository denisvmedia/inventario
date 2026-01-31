package jsonapi

import (
	"context"
	"net/http"

	"github.com/go-extras/errx"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

type RestoreOperationResponse struct {
	HTTPStatusCode int                           `json:"-"` // http response status code
	Data           *RestoreOperationResponseData `json:"data"`
}

// RestoreOperationResponseData is an object that holds restore operation information.
type RestoreOperationResponseData struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type" example:"restores" enums:"restores"`
	Attributes models.RestoreOperation `json:"attributes"`
}

type RestoreOperationsResponse struct {
	HTTPStatusCode int                             `json:"-"` // http response status code
	Data           []*RestoreOperationResponseData `json:"data"`
}

func NewRestoreOperationResponse(operation *models.RestoreOperation) *RestoreOperationResponse {
	return &RestoreOperationResponse{
		HTTPStatusCode: http.StatusOK,
		Data: &RestoreOperationResponseData{
			ID:         operation.ID,
			Type:       "restores",
			Attributes: *operation,
		},
	}
}

func NewRestoreOperationsResponse(operations []*models.RestoreOperation) *RestoreOperationsResponse {
	data := make([]*RestoreOperationResponseData, len(operations))
	for i, operation := range operations {
		data[i] = &RestoreOperationResponseData{
			ID:         operation.ID,
			Type:       "restores",
			Attributes: *operation,
		}
	}

	return &RestoreOperationsResponse{
		HTTPStatusCode: http.StatusOK,
		Data:           data,
	}
}

func (r *RestoreOperationResponse) Render(w http.ResponseWriter, req *http.Request) error {
	r.HTTPStatusCode = http.StatusOK
	return nil
}

func (r *RestoreOperationsResponse) Render(w http.ResponseWriter, req *http.Request) error {
	r.HTTPStatusCode = http.StatusOK
	return nil
}

type RestoreOperationCreateRequest struct {
	Data *RestoreOperationCreateRequestData `json:"data"`
}

// RestoreOperationCreateRequestData is an object that holds restore operation creation information.
type RestoreOperationCreateRequestData struct {
	Type       string                   `json:"type" example:"restores" enums:"restores"`
	Attributes *models.RestoreOperation `json:"attributes"`
}

func (a *RestoreOperationCreateRequest) Bind(r *http.Request) error {
	if a.Data == nil {
		return errx.Wrap("missing required data field", nil)
	}

	if a.Data.Type != "restores" {
		return errx.Wrap("invalid type, expected 'restores'", nil)
	}

	if a.Data.Attributes == nil {
		return errx.Wrap("missing required attributes field", nil)
	}

	return a.validateAttributesWithContext(r.Context())
}

func (a *RestoreOperationCreateRequest) validateAttributesWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.Data.Attributes.Description, validation.Required, validation.Length(1, 500)),
		validation.Field(&a.Data.Attributes.Options),
	)

	return validation.ValidateStructWithContext(ctx, a.Data.Attributes, fields...)
}

type RestoreOperationUpdateRequest struct {
	Data *RestoreOperationUpdateRequestData `json:"data"`
}

// RestoreOperationUpdateRequestData is an object that holds restore operation update information.
type RestoreOperationUpdateRequestData struct {
	Type       string                   `json:"type" example:"restores" enums:"restores"`
	Attributes *models.RestoreOperation `json:"attributes"`
}

func (a *RestoreOperationUpdateRequest) Bind(r *http.Request) error {
	if a.Data == nil {
		return errx.Wrap("missing required data field", nil)
	}

	if a.Data.Type != "restores" {
		return errx.Wrap("invalid type, expected 'restores'", nil)
	}

	if a.Data.Attributes == nil {
		return errx.Wrap("missing required attributes field", nil)
	}

	return a.validateAttributesWithContext(r.Context())
}

func (a *RestoreOperationUpdateRequest) validateAttributesWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.Data.Attributes.Description, validation.Required, validation.Length(1, 500)),
		validation.Field(&a.Data.Attributes.Options),
	)

	return validation.ValidateStructWithContext(ctx, a.Data.Attributes, fields...)
}
