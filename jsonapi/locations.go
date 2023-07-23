package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

type Location struct {
	*models.Location
	Areas []string `json:"areas"`
}

type LocationResponse struct {
	HTTPStatusCode int                   `json:"-"` // http response status code
	Data           *LocationResponseData `json:"data"`
}

// LocationResponseData is an object that holds location information.
type LocationResponseData struct {
	ID         string    `json:"id"`
	Type       string    `json:"type" example:"locations" enums:"locations"`
	Attributes *Location `json:"attributes"`
}

func NewLocationResponse(location *Location) *LocationResponse {
	return &LocationResponse{
		Data: &LocationResponseData{
			ID:         location.ID,
			Type:       "locations",
			Attributes: location,
		},
	}
}

func (rd *LocationResponse) WithStatusCode(statusCode int) *LocationResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *LocationResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

// LocationsMeta is a meta information for LocationsResponse.
type LocationsMeta struct {
	Locations int `json:"locations" example:"1" format:"int64"`
}

// LocationsResponse is an object that holds location list information.
type LocationsResponse struct {
	Data []models.Location `json:"data"`
	Meta LocationsMeta     `json:"meta"`
}

func NewLocationsResponse(locations []models.Location, total int) *LocationsResponse {
	return &LocationsResponse{
		Data: locations,
		Meta: LocationsMeta{
			Locations: total,
		},
	}
}

func (rd *LocationsResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

var _ render.Binder = (*LocationRequest)(nil)

type LocationRequest struct {
	Data *LocationRequestData `json:"data"`
}

// LocationRequestData is an object that holds location data information.
type LocationRequestData struct {
	Type       string           `json:"type" example:"locations" enums:"locations"`
	Attributes *models.Location `json:"attributes"`
}

func (lr *LocationRequestData) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&lr.Type, validation.Required, validation.In("locations")),
		validation.Field(&lr.Attributes, validation.Required),
	)
	return validation.ValidateStruct(lr, fields...)
}

func (lr *LocationRequest) Bind(r *http.Request) error {
	err := lr.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (lr *LocationRequest) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&lr.Data, validation.Required),
	)
	return validation.ValidateStruct(lr, fields...)
}
