package jsonapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// ItemTypeView is an item type as the API shows it. Slug is what an item's
// type field holds and never changes. Name is empty for a built-in type that
// keeps its default name, which clients translate from the slug.
type ItemTypeView struct {
	Slug      string `json:"slug" example:"electronics"`
	Name      string `json:"name" example:""`
	Icon      string `json:"icon" example:"laptop"`
	Builtin   bool   `json:"builtin" example:"true"`
	ItemCount int    `json:"item_count" example:"3" format:"int64"`
	// TrashedItemCount is the number of items in Trash with this type.
	// Deleting the type moves them along with the others.
	TrashedItemCount int `json:"trashed_item_count" example:"0" format:"int64"`
	// DraftCount is the number of unfinished drafts with this type.
	// Deleting the type moves them along with the items.
	DraftCount int       `json:"draft_count" example:"0" format:"int64"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func newItemTypeView(t *models.ItemType, usage registry.ItemTypeUsage) ItemTypeView {
	return ItemTypeView{
		Slug:             string(t.Slug),
		Name:             t.Name,
		Icon:             t.Icon,
		Builtin:          t.Builtin(),
		ItemCount:        usage.Live,
		TrashedItemCount: usage.Trashed,
		DraftCount:       usage.Drafts,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
	}
}

// ItemTypeResponse is the JSON:API envelope for one item type.
type ItemTypeResponse struct {
	HTTPStatusCode int                   `json:"-"`
	Data           *ItemTypeResponseData `json:"data"`
}

// ItemTypeResponseData is the inner resource object.
type ItemTypeResponseData struct {
	ID         string       `json:"id"`
	Type       string       `json:"type" example:"item_types" enums:"item_types"`
	Attributes ItemTypeView `json:"attributes"`
}

func newItemTypeResponseData(t *models.ItemType, usage registry.ItemTypeUsage) *ItemTypeResponseData {
	return &ItemTypeResponseData{ID: t.ID, Type: "item_types", Attributes: newItemTypeView(t, usage)}
}

func NewItemTypeResponse(t *models.ItemType, usage registry.ItemTypeUsage) *ItemTypeResponse {
	return &ItemTypeResponse{Data: newItemTypeResponseData(t, usage)}
}

func (tr *ItemTypeResponse) WithStatusCode(code int) *ItemTypeResponse {
	tmp := *tr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (tr *ItemTypeResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(tr.HTTPStatusCode, http.StatusOK))
	return nil
}

// ItemTypesMeta is the count block on a list response.
type ItemTypesMeta struct {
	ItemTypes int `json:"item_types" example:"6" format:"int64"`
}

// ItemTypesResponse lists the item types of a group: the built-in ones in
// their fixed order, then the others by name.
type ItemTypesResponse struct {
	Data []*ItemTypeResponseData `json:"data"`
	Meta ItemTypesMeta           `json:"meta"`
}

// NewItemTypesResponse wraps types; counts maps a slug to its item counts,
// and a type missing from it has no items.
func NewItemTypesResponse(types []*models.ItemType, counts map[models.CommodityType]registry.ItemTypeUsage) *ItemTypesResponse {
	data := make([]*ItemTypeResponseData, 0, len(types))
	for _, t := range types {
		data = append(data, newItemTypeResponseData(t, counts[t.Slug]))
	}
	return &ItemTypesResponse{Data: data, Meta: ItemTypesMeta{ItemTypes: len(data)}}
}

func (*ItemTypesResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// ItemTypeRequest is the JSON:API payload for POST /item-types.
type ItemTypeRequest struct {
	Data *ItemTypeRequestDataWrapper `json:"data"`
}

type ItemTypeRequestDataWrapper struct {
	ID         string              `json:"id,omitempty"`
	Type       string              `json:"type" example:"item_types" enums:"item_types"`
	Attributes ItemTypeRequestData `json:"attributes"`
}

// ItemTypeRequestData carries the fields of a new item type. The slug is
// made from the name by the server. The handler normalizes and validates
// the fields with models.ItemType.ValidateFields.
type ItemTypeRequestData struct {
	Name string `json:"name" example:"Vehicle"`
	Icon string `json:"icon,omitempty" example:"car"`
}

func (w *ItemTypeRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if w.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In("item_types")),
	)
}

func (tr *ItemTypeRequest) Bind(r *http.Request) error {
	return tr.ValidateWithContext(r.Context())
}

func (tr *ItemTypeRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, tr,
		validation.Field(&tr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*ItemTypeRequest)(nil)
	_ validation.ValidatableWithContext = (*ItemTypeRequest)(nil)
	_ validation.ValidatableWithContext = (*ItemTypeRequestDataWrapper)(nil)
)

// ItemTypeUpdateRequest is the JSON:API payload for PATCH /item-types/{id}.
// An absent attribute is left unchanged. The slug cannot be changed.
type ItemTypeUpdateRequest struct {
	Data *ItemTypeUpdateRequestDataWrapper `json:"data"`
}

type ItemTypeUpdateRequestDataWrapper struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type" example:"item_types" enums:"item_types"`
	Attributes ItemTypeUpdateRequestData `json:"attributes"`
}

// ItemTypeUpdateRequestData carries the fields to change. An empty name on
// a built-in type gives it back its translated default name.
type ItemTypeUpdateRequestData struct {
	Name *string `json:"name,omitempty"`
	Icon *string `json:"icon,omitempty"`
}

// Apply copies the attributes present in d onto t.
func (d ItemTypeUpdateRequestData) Apply(t *models.ItemType) {
	if d.Name != nil {
		t.Name = *d.Name
	}
	if d.Icon != nil {
		t.Icon = *d.Icon
	}
}

func (w *ItemTypeUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In("item_types")),
	)
}

func (ur *ItemTypeUpdateRequest) Bind(r *http.Request) error {
	return ur.ValidateWithContext(r.Context())
}

func (ur *ItemTypeUpdateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, ur,
		validation.Field(&ur.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*ItemTypeUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*ItemTypeUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*ItemTypeUpdateRequestDataWrapper)(nil)
)

// ItemTypeDeleteResponse reports how many items a delete moved to the type
// named by reassign_to.
type ItemTypeDeleteResponse struct {
	Meta ItemTypeDeleteMeta `json:"meta"`
}

type ItemTypeDeleteMeta struct {
	ReassignedItems int `json:"reassigned_items" example:"4" format:"int64"`
}

func (*ItemTypeDeleteResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
