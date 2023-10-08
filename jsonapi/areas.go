package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

type AreaResponse struct {
	HTTPStatusCode int               `json:"-"` // http response status code
	Data           *AreaResponseData `json:"data"`
}

// AreaResponseData is an object that holds area information.
type AreaResponseData struct {
	ID         string      `json:"id"`
	Type       string      `json:"type" example:"areas" enums:"areas"`
	Attributes models.Area `json:"attributes"`
}

func NewAreaResponse(area *models.Area) *AreaResponse {
	return &AreaResponse{
		Data: &AreaResponseData{
			ID:         area.ID,
			Type:       "areas",
			Attributes: *area,
		},
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
	Data []AreaData `json:"data"`
	Meta AreasMeta  `json:"meta"`
}

func NewAreasResponse(areas []*models.Area, total int) *AreasResponse {
	areaData := make([]AreaData, 0) // must be an empty array instead of nil due to JSON serialization
	for _, l := range areas {
		l := *l
		areaData = append(areaData, AreaData{
			ID:         l.ID,
			Type:       "areas",
			Attributes: &l,
		})
	}

	return &AreasResponse{
		Data: areaData,
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
	Data *AreaData `json:"data"`
}

// AreaData is an object that holds area data information.
type AreaData struct {
	ID         string       `json:"id,omitempty"`
	Type       string       `json:"type" example:"areas" enums:"areas"`
	Attributes *models.Area `json:"attributes"`
}

func (lr *AreaData) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&lr.Type, validation.Required, validation.In("areas")),
		validation.Field(&lr.Attributes, validation.Required),
	)
	return validation.ValidateStruct(lr, fields...)
}

func (lr *AreaRequest) Bind(r *http.Request) error {
	err := lr.Validate()
	if err != nil {
		return err
	}

	lr.Data.Attributes.ID = lr.Data.ID

	return nil
}

func (lr *AreaRequest) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&lr.Data, validation.Required),
	)
	return validation.ValidateStruct(lr, fields...)
}
