package jsonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
)

// SellerView is a seller with the number of the group's items that point at
// it, drafts and inactive items included and items in Trash left out.
type SellerView struct {
	*models.Seller
	ItemCount int `json:"item_count" example:"3" format:"int64"`
}

// SellerResponse is the JSON:API envelope for one seller.
type SellerResponse struct {
	HTTPStatusCode int                 `json:"-"`
	Data           *SellerResponseData `json:"data"`
}

// SellerResponseData is the inner resource object.
type SellerResponseData struct {
	ID         string     `json:"id"`
	Type       string     `json:"type" example:"sellers" enums:"sellers"`
	Attributes SellerView `json:"attributes"`
}

func NewSellerResponse(s *models.Seller, itemCount int) *SellerResponse {
	return &SellerResponse{
		Data: &SellerResponseData{
			ID:         s.ID,
			Type:       "sellers",
			Attributes: SellerView{Seller: s, ItemCount: itemCount},
		},
	}
}

func (sr *SellerResponse) WithStatusCode(code int) *SellerResponse {
	tmp := *sr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (sr *SellerResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(sr.HTTPStatusCode, http.StatusOK))
	return nil
}

// SellersMeta is the count block on a list response.
type SellersMeta struct {
	Sellers int `json:"sellers" example:"12" format:"int64"`
}

// SellersResponse lists the sellers of a group, sorted by name.
type SellersResponse struct {
	Data []*SellerResponseData `json:"data"`
	Meta SellersMeta           `json:"meta"`
}

// NewSellersResponse wraps sellers; counts maps a seller ID to its item
// count, and a seller missing from it has no items.
func NewSellersResponse(sellers []*models.Seller, counts map[string]int) *SellersResponse {
	data := make([]*SellerResponseData, 0, len(sellers))
	for _, s := range sellers {
		data = append(data, &SellerResponseData{
			ID:         s.ID,
			Type:       "sellers",
			Attributes: SellerView{Seller: s, ItemCount: counts[s.ID]},
		})
	}
	return &SellersResponse{Data: data, Meta: SellersMeta{Sellers: len(data)}}
}

func (*SellersResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// SellerRequest is the JSON:API payload for POST /sellers.
type SellerRequest struct {
	Data *SellerRequestDataWrapper `json:"data"`
}

type SellerRequestDataWrapper struct {
	ID         string            `json:"id,omitempty"`
	Type       string            `json:"type" example:"sellers" enums:"sellers"`
	Attributes SellerRequestData `json:"attributes"`
}

// SellerRequestData carries the fields of a new seller. The handler
// normalizes and validates them with models.Seller.ValidateFields.
type SellerRequestData struct {
	Name    string `json:"name" example:"Hardware Store"`
	Website string `json:"website,omitempty" example:"https://hardware.example"`
	Contact string `json:"contact,omitempty" example:"orders@hardware.example"`
	Note    string `json:"note,omitempty"`
}

func (w *SellerRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if w.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In("sellers")),
	)
}

func (sr *SellerRequest) Bind(r *http.Request) error {
	return sr.ValidateWithContext(r.Context())
}

func (sr *SellerRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sr,
		validation.Field(&sr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*SellerRequest)(nil)
	_ validation.ValidatableWithContext = (*SellerRequest)(nil)
	_ validation.ValidatableWithContext = (*SellerRequestDataWrapper)(nil)
)

// SellerUpdateRequest is the JSON:API payload for PATCH /sellers/{id}. An
// absent attribute is left unchanged; the merged seller is validated as a
// whole.
type SellerUpdateRequest struct {
	Data *SellerUpdateRequestDataWrapper `json:"data"`
}

type SellerUpdateRequestDataWrapper struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type" example:"sellers" enums:"sellers"`
	Attributes SellerUpdateRequestData `json:"attributes"`
}

type SellerUpdateRequestData struct {
	Name    *string `json:"name,omitempty"`
	Website *string `json:"website,omitempty"`
	Contact *string `json:"contact,omitempty"`
	Note    *string `json:"note,omitempty"`
}

// Apply copies the attributes present in d onto s.
func (d SellerUpdateRequestData) Apply(s *models.Seller) {
	if d.Name != nil {
		s.Name = *d.Name
	}
	if d.Website != nil {
		s.Website = *d.Website
	}
	if d.Contact != nil {
		s.Contact = *d.Contact
	}
	if d.Note != nil {
		s.Note = *d.Note
	}
}

func (w *SellerUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In("sellers")),
	)
}

func (ur *SellerUpdateRequest) Bind(r *http.Request) error {
	return ur.ValidateWithContext(r.Context())
}

func (ur *SellerUpdateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, ur,
		validation.Field(&ur.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*SellerUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*SellerUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*SellerUpdateRequestDataWrapper)(nil)
)
