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
	Meta           *CommodityResponseMeta `json:"meta,omitempty"`
}

// CommodityResponseData is an object that holds commodity information.
type CommodityResponseData struct {
	ID         string            `json:"id"`
	Type       string            `json:"type" example:"commodities" enums:"commodities"`
	Attributes *models.Commodity `json:"attributes"`
}

// CommodityResponseMeta carries per-resource derived data that does not
// belong on the model itself. `Cover` mirrors the `meta.covers[id]` slot
// on the list response so single-commodity callers see the same shape.
type CommodityResponseMeta struct {
	Cover *CommodityCover `json:"cover,omitempty"`
}

// CommodityCover is the resolved cover image for a commodity. `Source`
// distinguishes the (A) auto-pick path ("first_photo") from the (B)
// explicit-override path that lands later — the FE can use it to decide
// whether the star toggle is "set" or "auto".
type CommodityCover struct {
	FileID     string            `json:"file_id"`
	Thumbnails map[string]string `json:"thumbnails,omitempty"`
	Source     string            `json:"source" example:"first_photo" enums:"first_photo,explicit"`
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

// WithCover attaches the resolved cover image to the response. Returns a
// shallow copy so callers can chain after WithStatusCode.
func (cr *CommodityResponse) WithCover(cover *CommodityCover) *CommodityResponse {
	if cover == nil {
		return cr
	}
	tmp := *cr
	tmp.Meta = &CommodityResponseMeta{Cover: cover}
	return &tmp
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

	// Covers maps commodity id → resolved cover image. Empty (omitted) for
	// commodities without an attached photo. The FE renders the largest
	// thumbnail that fits the slot and falls back to the type emoji when
	// the entry is absent. Issue #1451 ships option (A) — first photo by
	// `created_at` ASC; option (B) `cover_file_id` override is filed as a
	// follow-up sub-issue.
	Covers map[string]CommodityCover `json:"covers,omitempty"`
}

// CommoditiesResponse is an object that holds a list of commodities information.
type CommoditiesResponse struct {
	Data []CommodityData `json:"data"`
	Meta CommoditiesMeta `json:"meta"`
}

// NewCommoditiesResponse creates a new CommoditiesResponse instance with pagination metadata.
func NewCommoditiesResponse(commodities []*models.Commodity, total, page, perPage int) *CommoditiesResponse {
	return NewCommoditiesResponseWithCovers(commodities, total, page, perPage, nil)
}

// NewCommoditiesResponseWithCovers is NewCommoditiesResponse plus a per-id
// cover map embedded under `meta.covers`. Pass nil when the caller cannot
// compute covers (e.g. error path or unauthenticated context); the FE
// already handles the absent-cover fallback.
func NewCommoditiesResponseWithCovers(commodities []*models.Commodity, total, page, perPage int, covers map[string]CommodityCover) *CommoditiesResponse {
	commodityData := make([]CommodityData, 0) // must be an empty array instead of nil due to JSON serialization
	for _, l := range commodities {
		l := *l
		commodityData = append(commodityData, CommodityData{
			ID:         l.ID,
			Type:       "commodities",
			Attributes: &l,
		})
	}

	meta := CommoditiesMeta{
		Commodities: total,
		Page:        page,
		PerPage:     perPage,
		TotalPages:  ComputeTotalPages(total, perPage),
	}
	if len(covers) > 0 {
		meta.Covers = covers
	}

	return &CommoditiesResponse{
		Data: commodityData,
		Meta: meta,
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

// CommodityCoverRequest is the body for `PATCH /commodities/{id}/cover`
// (issue #1451 option B). The `attributes.file_id` field is a JSON-API-
// style PATCH: a non-empty string sets the explicit override; null or an
// empty string clears it (the resolver falls back to the first photo).
type CommodityCoverRequest struct {
	Data *CommodityCoverRequestData `json:"data"`
}

// CommodityCoverRequestData is the payload of a CommodityCoverRequest.
type CommodityCoverRequestData struct {
	Type       string                          `json:"type" example:"commodity_cover" enums:"commodity_cover"`
	Attributes CommodityCoverRequestAttributes `json:"attributes"`
}

// CommodityCoverRequestAttributes carries the only mutable field on the
// cover endpoint: the file id. `*string` distinguishes "clear the
// override" (null / empty) from "leave alone" — though the endpoint is
// always a write, so an absent value is treated the same as null.
type CommodityCoverRequestAttributes struct {
	FileID *string `json:"file_id"`
}

var _ render.Binder = (*CommodityCoverRequest)(nil)

// Bind validates the request body. The endpoint is intentionally narrow
// — only `data.type` and `data.attributes.file_id` are read; anything
// else is ignored.
func (ccr *CommodityCoverRequest) Bind(_ *http.Request) error {
	return ccr.Validate()
}

// Validate enforces the JSON-API envelope shape. The file_id value
// itself is validated by the handler against the live commodity (to
// confirm the file belongs to this commodity and is an image), since
// that check needs a registry call.
func (ccr *CommodityCoverRequest) Validate() error {
	if ccr.Data == nil {
		return validation.NewError("data_required", "data is required")
	}
	return validation.ValidateStruct(ccr.Data,
		validation.Field(&ccr.Data.Type, validation.Required, validation.In("commodity_cover")),
	)
}
