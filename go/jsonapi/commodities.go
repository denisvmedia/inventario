// Package jsonapi provides JSON API responses and request binding for commodities.
package jsonapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

type CommodityMeta struct {
	Images        []string `json:"images"`
	Manuals       []string `json:"manuals"`
	Invoices      []string `json:"invoices"`
	ImagesError   string   `json:"images_error,omitempty"`
	ManualsError  string   `json:"manuals_error,omitempty"`
	InvoicesError string   `json:"invoices_error,omitempty"`
}

func (a *CommodityMeta) MarshalJSON() ([]byte, error) {
	tmp := *a
	if tmp.Images == nil {
		tmp.Images = make([]string, 0)
	}
	if tmp.Manuals == nil {
		tmp.Manuals = make([]string, 0)
	}
	if tmp.Invoices == nil {
		tmp.Invoices = make([]string, 0)
	}
	return json.Marshal(tmp)
}

// CommodityResponse is an object that holds commodity information.
type CommodityResponse struct {
	HTTPStatusCode int                    `json:"-"` // HTTP response status code
	Data           *CommodityResponseData `json:"data"`
}

// CommodityResponseData is an object that holds commodity information.
type CommodityResponseData struct {
	ID         string            `json:"id"`
	Type       string            `json:"type" example:"commodities" enums:"commodities"`
	Attributes *models.Commodity `json:"attributes"`
	Meta       *CommodityMeta    `json:"meta"`
}

// NewCommodityResponse creates a new CommodityResponse instance.
func NewCommodityResponse(commodity *models.Commodity, meta *CommodityMeta) *CommodityResponse {
	return &CommodityResponse{
		Data: &CommodityResponseData{
			ID:         commodity.ID,
			Type:       "commodities",
			Attributes: commodity,
			Meta:       meta,
		},
	}
}

// WithStatusCode sets the HTTP response status code for the CommodityResponse.
func (cr *CommodityResponse) WithStatusCode(statusCode int) *CommodityResponse {
	tmp := *cr
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the CommodityResponse as an HTTP response.
func (cr *CommodityResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(cr.HTTPStatusCode, http.StatusOK))
	return nil
}

// CommoditiesMeta is a meta information for CommoditiesResponse.
type CommoditiesMeta struct {
	Commodities int `json:"commodities" example:"1" format:"int64"`
}

// CommoditiesResponse is an object that holds a list of commodities information.
type CommoditiesResponse struct {
	Data []CommodityData `json:"data"`
	Meta CommoditiesMeta `json:"meta"`
}

// NewCommoditiesResponse creates a new CommoditiesResponse instance.
func NewCommoditiesResponse(commodities []*models.Commodity, total int) *CommoditiesResponse {
	commodityData := make([]CommodityData, 0) // must be an empty array instead of nil due to JSON serialization
	for _, l := range commodities {
		l := *l
		commodityData = append(commodityData, CommodityData{
			ID:         l.ID,
			Type:       "commodities",
			Attributes: &l,
		})
	}

	return &CommoditiesResponse{
		Data: commodityData,
		Meta: CommoditiesMeta{Commodities: total},
	}
}

// Render renders the CommoditiesResponse as an HTTP response.
func (*CommoditiesResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CommodityRequest is an object that holds commodity data information.
type CommodityRequest struct {
	Data *CommodityData `json:"data"`
}

// CommodityData is an object that holds commodity data information.
type CommodityData struct {
	ID         string            `json:"id,omitempty"`
	Type       string            `json:"type" example:"commodities" enums:"commodities"`
	Attributes *models.Commodity `json:"attributes"`
}

func (cd *CommodityData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cd.Type, validation.Required, validation.In("commodities")),
		validation.Field(&cd.Attributes, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, cd, fields...)
}

var _ render.Binder = (*CommodityRequest)(nil)

// Bind binds the commodity data from the request to the CommodityRequest object.
func (cr *CommodityRequest) Bind(r *http.Request) error {
	err := cr.ValidateWithContext(r.Context())
	if err != nil {
		return err
	}

	return nil
}

// ValidateWithContext validates the commodity request data.
func (cr *CommodityRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&cr.Data, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, cr, fields...)
}
