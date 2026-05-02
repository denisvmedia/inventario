// Package jsonapi provides JSON API responses and request binding for commodities.
package jsonapi

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

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
}

// NewCommodityResponse creates a new CommodityResponse instance. The legacy
// `meta.{images,manuals,invoices}` arrays were removed under #1421 — the
// unified `/files?linked_entity_type=commodity&linked_entity_id=<id>` query
// is the source of truth for commodity attachments.
func NewCommodityResponse(commodity *models.Commodity) *CommodityResponse {
	return &CommodityResponse{
		Data: &CommodityResponseData{
			ID:         commodity.ID,
			Type:       "commodities",
			Attributes: commodity,
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
	Page        int `json:"page" example:"1" format:"int64"`
	PerPage     int `json:"per_page" example:"50" format:"int64"`
	TotalPages  int `json:"total_pages" example:"1" format:"int64"`
}

// CommoditiesResponse is an object that holds a list of commodities information.
type CommoditiesResponse struct {
	Data []CommodityData `json:"data"`
	Meta CommoditiesMeta `json:"meta"`
}

// NewCommoditiesResponse creates a new CommoditiesResponse instance with pagination metadata.
func NewCommoditiesResponse(commodities []*models.Commodity, total, page, perPage int) *CommoditiesResponse {
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
		Meta: CommoditiesMeta{
			Commodities: total,
			Page:        page,
			PerPage:     perPage,
			TotalPages:  ComputeTotalPages(total, perPage),
		},
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

	// Only reject ID fields in CREATE requests (POST), allow them in UPDATE requests (PUT)
	if httpMethod, ok := ctx.Value(httpMethodKey).(string); ok && httpMethod == "POST" {
		fields = append(fields,
			validation.Field(&cd.ID, validation.Empty.Error("ID field not allowed in create requests")),
		)
	}

	return validation.ValidateStructWithContext(ctx, cd, fields...)
}

var _ render.Binder = (*CommodityRequest)(nil)

// Bind binds the commodity data from the request to the CommodityRequest object.
func (cr *CommodityRequest) Bind(r *http.Request) error {
	// Add HTTP method to context for validation
	ctx := context.WithValue(r.Context(), httpMethodKey, r.Method)
	err := cr.ValidateWithContext(ctx)
	if err != nil {
		return err
	}

	// For UPDATE requests, set the ID from the request data
	if r.Method == "PUT" && cr.Data.ID != "" {
		cr.Data.Attributes.ID = cr.Data.ID
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
