// Package jsonapi provides JSON API responses and request binding for commodities.
package jsonapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

type Commodity struct {
	*models.Commodity
	CommodityExtra
}

type CommodityExtra struct {
	Images   []string `json:"images"`
	Manuals  []string `json:"manuals"`
	Invoices []string `json:"invoices"`
}

func (a *Commodity) MarshalJSON() ([]byte, error) {
	type AliasCommodity models.Commodity
	type Alias struct {
		*AliasCommodity
		CommodityExtra
	}
	tmp := AliasCommodity(*a.Commodity)
	var x = Alias{
		AliasCommodity: &tmp,
		CommodityExtra: a.CommodityExtra,
	}
	v, err := json.Marshal(x)
	return v, err
}

// CommodityResponse is an object that holds commodity information.
type CommodityResponse struct {
	HTTPStatusCode int `json:"-"` // HTTP response status code

	ID         string     `json:"id"`
	Type       string     `json:"type" example:"commodities" enums:"commodities"`
	Attributes *Commodity `json:"attributes"`
}

// NewCommodityResponse creates a new CommodityResponse instance.
func NewCommodityResponse(commodity *Commodity) *CommodityResponse {
	return &CommodityResponse{
		ID:         commodity.ID,
		Type:       "commodities",
		Attributes: commodity,
	}
}

// WithStatusCode sets the HTTP response status code for the CommodityResponse.
func (cr *CommodityResponse) WithStatusCode(statusCode int) *CommodityResponse {
	tmp := *cr
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the CommodityResponse as an HTTP response.
func (cr *CommodityResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(cr.HTTPStatusCode, http.StatusOK))
	return nil
}

// CommoditiesMeta is a meta information for CommoditiesResponse.
type CommoditiesMeta struct {
	Commodities int `json:"commodities" example:"1" format:"int64"`
}

// CommoditiesResponse is an object that holds a list of commodities information.
type CommoditiesResponse struct {
	Data []models.Commodity `json:"data"`
	Meta CommoditiesMeta    `json:"meta"`
}

// NewCommoditiesResponse creates a new CommoditiesResponse instance.
func NewCommoditiesResponse(commodities []models.Commodity, total int) *CommoditiesResponse {
	return &CommoditiesResponse{
		Data: commodities,
		Meta: CommoditiesMeta{Commodities: total},
	}
}

// Render renders the CommoditiesResponse as an HTTP response.
func (cr *CommoditiesResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CommodityRequest is an object that holds commodity data information.
type CommodityRequest struct {
	Data *models.Commodity `json:"data"`
}

var _ render.Binder = (*CommodityRequest)(nil)

// Bind binds the commodity data from the request to the CommodityRequest object.
func (cr *CommodityRequest) Bind(r *http.Request) error {
	err := cr.Validate()
	if err != nil {
		return err
	}

	return nil
}

// Validate validates the commodity request data.
func (cr *CommodityRequest) Validate() error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cr.Data, validation.Required),
	)
	return validation.ValidateStruct(cr, fields...)
}
