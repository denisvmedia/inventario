package jsonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

// SupplyLinkResponse is the single-resource JSON:API envelope for a
// supply link (#1369). Mirrors the project-wide `{data: {id, type,
// attributes}}` shape used by commodities, areas, loans, ...
type SupplyLinkResponse struct {
	HTTPStatusCode int                     `json:"-"`
	Data           *SupplyLinkResponseData `json:"data"`
}

// SupplyLinkResponseData is the inner resource object.
type SupplyLinkResponseData struct {
	ID         string            `json:"id"`
	Type       string            `json:"type" example:"commodity_supply_links" enums:"commodity_supply_links"`
	Attributes models.SupplyLink `json:"attributes"`
}

// NewSupplyLinkResponse wraps a single supply link into the envelope.
func NewSupplyLinkResponse(link *models.SupplyLink) *SupplyLinkResponse {
	return &SupplyLinkResponse{
		Data: &SupplyLinkResponseData{
			ID:         link.ID,
			Type:       "commodity_supply_links",
			Attributes: *link,
		},
	}
}

// WithStatusCode is the shared "I want to render 201 not 200" helper
// pattern; matches commodity_loans / commodity_services.
func (sr *SupplyLinkResponse) WithStatusCode(code int) *SupplyLinkResponse {
	tmp := *sr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (sr *SupplyLinkResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(sr.HTTPStatusCode, http.StatusOK))
	return nil
}

// SupplyLinksMeta describes pagination metadata on a list response.
// Supply links don't paginate (per-commodity list is bounded), but we
// keep the meta shape consistent with loans so FE list-rendering
// helpers stay uniform.
type SupplyLinksMeta struct {
	SupplyLinks int `json:"supply_links" example:"3" format:"int64"`
	Total       int `json:"total" example:"3" format:"int64"`
}

// SupplyLinksResponse is the per-commodity list envelope.
type SupplyLinksResponse struct {
	Data []*models.SupplyLink `json:"data"`
	Meta SupplyLinksMeta      `json:"meta"`
}

// NewSupplyLinksResponse wraps a slice into the list envelope.
func NewSupplyLinksResponse(links []*models.SupplyLink, total int) *SupplyLinksResponse {
	return &SupplyLinksResponse{
		Data: links,
		Meta: SupplyLinksMeta{SupplyLinks: len(links), Total: total},
	}
}

func (*SupplyLinksResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// SupplyLinkRequest is the JSON:API payload for POST .../supplies.
type SupplyLinkRequest struct {
	Data *SupplyLinkRequestDataWrapper `json:"data"`
}

type SupplyLinkRequestDataWrapper struct {
	ID         string                `json:"id,omitempty"`
	Type       string                `json:"type" example:"commodity_supply_links" enums:"commodity_supply_links"`
	Attributes SupplyLinkRequestData `json:"attributes"`
}

// SupplyLinkRequestData carries the user-supplied fields on create.
type SupplyLinkRequestData struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Notes string `json:"notes,omitempty"`
}

func (srd *SupplyLinkRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (srd *SupplyLinkRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, srd,
		validation.Field(&srd.Label, validation.Required, validation.Length(1, 200)),
		validation.Field(&srd.URL, validation.Required, validation.Length(1, 2048)),
		validation.Field(&srd.Notes, validation.Length(0, 1000)),
	)
}

func (srdw *SupplyLinkRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if srdw.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, srdw,
		validation.Field(&srdw.Type, validation.Required, validation.In("commodity_supply_links")),
		validation.Field(&srdw.Attributes, validation.Required),
	)
}

func (sr *SupplyLinkRequest) Bind(r *http.Request) error {
	return sr.ValidateWithContext(r.Context())
}

func (sr *SupplyLinkRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sr,
		validation.Field(&sr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*SupplyLinkRequest)(nil)
	_ validation.ValidatableWithContext = (*SupplyLinkRequest)(nil)
	_ validation.ValidatableWithContext = (*SupplyLinkRequestDataWrapper)(nil)
	_ validation.ValidatableWithContext = (*SupplyLinkRequestData)(nil)
)

// SupplyLinkUpdateRequest is the JSON:API payload for PATCH
// .../supplies/{id}. All fields are pointers — nil means "leave
// unchanged"; non-nil (even empty string) means "set to this value".
// There is no tri-state need here: an empty Notes string IS the
// "clear notes" wire form, since Notes is a non-null TEXT column with
// "" as the default.
type SupplyLinkUpdateRequest struct {
	Data *SupplyLinkUpdateRequestDataWrapper `json:"data"`
}

type SupplyLinkUpdateRequestDataWrapper struct {
	ID         string                      `json:"id"`
	Type       string                      `json:"type" example:"commodity_supply_links" enums:"commodity_supply_links"`
	Attributes SupplyLinkUpdateRequestData `json:"attributes"`
}

type SupplyLinkUpdateRequestData struct {
	Label *string `json:"label,omitempty"`
	URL   *string `json:"url,omitempty"`
	Notes *string `json:"notes,omitempty"`
}

func (surd *SupplyLinkUpdateRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (surd *SupplyLinkUpdateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0, 3)
	if surd.Label != nil {
		fields = append(fields, validation.Field(surd.Label, validation.Required, validation.Length(1, 200)))
	}
	if surd.URL != nil {
		fields = append(fields, validation.Field(surd.URL, validation.Required, validation.Length(1, 2048)))
	}
	if surd.Notes != nil {
		fields = append(fields, validation.Field(surd.Notes, validation.Length(0, 1000)))
	}
	return validation.ValidateStructWithContext(ctx, surd, fields...)
}

func (sudw *SupplyLinkUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sudw,
		validation.Field(&sudw.Type, validation.Required, validation.In("commodity_supply_links")),
		validation.Field(&sudw.Attributes, validation.Required),
	)
}

func (sur *SupplyLinkUpdateRequest) Bind(r *http.Request) error {
	return sur.ValidateWithContext(r.Context())
}

func (sur *SupplyLinkUpdateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sur,
		validation.Field(&sur.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*SupplyLinkUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*SupplyLinkUpdateRequest)(nil)
)

// SupplyLinkReorderRequest is the JSON:API payload for POST
// .../commodities/{commodityID}/supplies/reorder. The IDs list IS the
// new visible order; the BE renumbers sort_order 0..N-1 in place.
//
// Why POST not PATCH on the collection: PATCH on the collection would
// imply partial state replacement; this endpoint applies a permutation
// (no row creates/deletes) and the verb POST + path /reorder reads
// cleaner in the OpenAPI surface than a custom PATCH semantic.
type SupplyLinkReorderRequest struct {
	Data *SupplyLinkReorderRequestData `json:"data"`
}

type SupplyLinkReorderRequestData struct {
	Type       string                             `json:"type" example:"commodity_supply_links_reorder" enums:"commodity_supply_links_reorder"`
	Attributes SupplyLinkReorderRequestAttributes `json:"attributes"`
}

type SupplyLinkReorderRequestAttributes struct {
	IDs []string `json:"ids"`
}

func (sra *SupplyLinkReorderRequestAttributes) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (sra *SupplyLinkReorderRequestAttributes) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sra,
		validation.Field(&sra.IDs, validation.Required, validation.Each(validation.Required)),
	)
}

func (srrd *SupplyLinkReorderRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, srrd,
		validation.Field(&srrd.Type, validation.Required, validation.In("commodity_supply_links_reorder")),
		validation.Field(&srrd.Attributes, validation.Required),
	)
}

func (srr *SupplyLinkReorderRequest) Bind(r *http.Request) error {
	return srr.ValidateWithContext(r.Context())
}

func (srr *SupplyLinkReorderRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, srr,
		validation.Field(&srr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*SupplyLinkReorderRequest)(nil)
	_ validation.ValidatableWithContext = (*SupplyLinkReorderRequest)(nil)
	_ validation.ValidatableWithContext = (*SupplyLinkReorderRequestData)(nil)
	_ validation.ValidatableWithContext = (*SupplyLinkReorderRequestAttributes)(nil)
)
