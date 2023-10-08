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
	Data []LocationData `json:"data"`
	Meta LocationsMeta  `json:"meta"`
}

func NewLocationsResponse(locations []*models.Location, total int) *LocationsResponse {
	locationData := make([]LocationData, 0) // must be an empty array instead of nil due to JSON serialization
	for _, l := range locations {
		l := *l
		locationData = append(locationData, LocationData{
			ID:         l.ID,
			Type:       "locations",
			Attributes: &l,
		})
	}

	return &LocationsResponse{
		Data: locationData,
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
	Data *LocationData `json:"data"`
}

// LocationData is an object that holds location data information.
type LocationData struct {
	ID         string           `json:"id,omitempty"`
	Type       string           `json:"type" example:"locations" enums:"locations"`
	Attributes *models.Location `json:"attributes"`
}

func (ld *LocationData) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&ld.Type, validation.Required, validation.In("locations")),
		validation.Field(&ld.Attributes, validation.Required),
	)
	return validation.ValidateStruct(ld, fields...)
}

func (lr *LocationRequest) Bind(r *http.Request) error {
	err := lr.Validate()
	if err != nil {
		return err
	}

	lr.Data.Attributes.ID = lr.Data.ID

	return nil
}

func (lr *LocationRequest) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&lr.Data, validation.Required),
	)
	return validation.ValidateStruct(lr, fields...)
}
