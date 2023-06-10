package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

// AreaResponse is an object that holds area information.
type AreaResponse struct {
	HTTPStatusCode int `json:"-"` // http response status code

	ID         string      `json:"id"`
	Type       string      `json:"type" example:"areas" enums:"areas"`
	Attributes models.Area `json:"attributes"`
}

func NewAreaResponse(area *models.Area) *AreaResponse {
	return &AreaResponse{
		ID:         area.ID,
		Type:       "areas",
		Attributes: *area,
	}
}

func (rd *AreaResponse) WithStatusCode(statusCode int) *AreaResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *AreaResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

// AreasMeta is a meta information for AreasResponse.
type AreasMeta struct {
	Areas int `json:"areas" example:"1" format:"int64"`
}

// AreasResponse is an object that holds area list information.
type AreasResponse struct {
	Data []models.Area `json:"data"`
	Meta AreasMeta     `json:"meta"`
}

func NewAreasResponse(areas []models.Area, total int) *AreasResponse {
	return &AreasResponse{
		Data: areas,
		Meta: AreasMeta{Areas: total},
	}
}

func (rd *AreasResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

var _ render.Binder = (*AreaRequest)(nil)

// AreaRequest is an object that holds area data information.
type AreaRequest struct {
	Data *models.Area `json:"data"`
}

func (lr *AreaRequest) Bind(r *http.Request) error {
	err := lr.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (lr *AreaRequest) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&lr.Data, validation.Required),
	)
	return validation.ValidateStruct(lr, fields...)
}