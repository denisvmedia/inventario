package jsonapi

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// ItemSetItemTypeView is one item type of an item set (#1607) as the group
// holds it.
type ItemSetItemTypeView struct {
	// Key identifies the type within the set.
	Key string `json:"key" example:"coins"`
	// Name is the name of the group's type, which a member may have changed,
	// or the set's name for a type the group does not have.
	Name string `json:"name" example:"Coins"`
	Icon string `json:"icon" example:"coins"`
	// State is missing when the group has no such type and adding the set
	// creates it, added when the set created the type, own when it is a
	// member's type of the same name, which the set uses and never deletes,
	// and taken when another type of the set was renamed to its name, so
	// adding cannot create it.
	State string `json:"state" example:"added" enums:"missing,added,own,taken"`
	// TakenBy is the key of the set's type that has this type's name, for a
	// taken type.
	TakenBy string `json:"taken_by,omitempty" example:"coins"`
	// ItemTypeID is the ID of the group's type; absent while missing or
	// taken.
	ItemTypeID       string `json:"item_type_id,omitempty" example:"8f1c..."`
	ItemCount        int    `json:"item_count" example:"0" format:"int64"`
	TrashedItemCount int    `json:"trashed_item_count" example:"0" format:"int64"`
	// Removable is true when removing the set deletes the type.
	Removable bool `json:"removable" example:"true"`
	// KeepReason says why removing the set keeps a type the group has:
	// own (a member made it), in_use (items have it, in Trash or not) or
	// last_type (a group keeps at least one type). Absent when the type is
	// removable or missing.
	KeepReason string `json:"keep_reason,omitempty" example:"in_use" enums:"own,in_use,last_type"`
}

// ItemSetFieldView is one custom field of an item set as the group holds
// it.
type ItemSetFieldView struct {
	// Key identifies the field within the set.
	Key string `json:"key" example:"condition"`
	// Name is the name of the group's field, which a member may have
	// changed, or the set's name for a field the group does not have.
	Name string `json:"name" example:"Condition"`
	// FieldType is the type of the group's field, or the set's type for a
	// missing one.
	FieldType string `json:"field_type" example:"select"`
	// State is missing, added, own or taken, as for an item type.
	State string `json:"state" example:"added" enums:"missing,added,own,taken"`
	// TakenBy is the key of the set's field that has this field's name, for
	// a taken field.
	TakenBy string `json:"taken_by,omitempty" example:"signed"`
	// CustomFieldID is the ID of the group's field; absent while missing or
	// taken.
	CustomFieldID string `json:"custom_field_id,omitempty" example:"5d2e..."`
	// ItemCount and TrashedItemCount count the items with a value of the
	// field, outside Trash and in it.
	ItemCount        int `json:"item_count" example:"0" format:"int64"`
	TrashedItemCount int `json:"trashed_item_count" example:"0" format:"int64"`
	// Removable is true when removing the set deletes the field.
	Removable bool `json:"removable" example:"true"`
	// KeepReason says why removing the set keeps a field the group has:
	// own or in_use (items have a value of it). Absent when the field is
	// removable or missing.
	KeepReason string `json:"keep_reason,omitempty" example:"in_use" enums:"own,in_use"`
}

// ItemSetView is an item set as the group holds it.
type ItemSetView struct {
	// Complete is true when adding the set has nothing left to create: the
	// group has every type and field of the set but those taken.
	Complete bool `json:"complete" example:"true"`
	// ItemTypes and CustomFields list the set's entries in the set's order.
	ItemTypes    []ItemSetItemTypeView `json:"item_types"`
	CustomFields []ItemSetFieldView    `json:"custom_fields"`
}

func newItemSetView(st *services.ItemSetStatus) ItemSetView {
	v := ItemSetView{
		Complete:     st.Complete(),
		ItemTypes:    make([]ItemSetItemTypeView, 0, len(st.ItemTypes)),
		CustomFields: make([]ItemSetFieldView, 0, len(st.Fields)),
	}
	for _, t := range st.ItemTypes {
		row := ItemSetItemTypeView{
			Key:   t.Entry.Key,
			Name:  t.Entry.Name,
			Icon:    t.Entry.Icon,
			State:   string(t.State),
			TakenBy: t.TakenBy,
		}
		if t.ItemType != nil {
			row.Name = t.ItemType.DisplayName()
			row.Icon = t.ItemType.Icon
			row.ItemTypeID = t.ItemType.ID
			row.ItemCount = t.Usage.Live
			row.TrashedItemCount = t.Usage.Trashed
			row.Removable = t.Removable()
			row.KeepReason = string(t.KeepReason)
		}
		v.ItemTypes = append(v.ItemTypes, row)
	}
	for _, f := range st.Fields {
		row := ItemSetFieldView{
			Key:       f.Entry.Key,
			Name:      f.Entry.Name,
			FieldType: string(f.Entry.Type),
			State:     string(f.State),
			TakenBy:   f.TakenBy,
		}
		if f.Field != nil {
			row.Name = f.Field.Name
			row.FieldType = string(f.Field.Type)
			row.CustomFieldID = f.Field.ID
			row.ItemCount = f.Usage.Live
			row.TrashedItemCount = f.Usage.Trashed
			row.Removable = f.Removable()
			row.KeepReason = string(f.KeepReason)
		}
		v.CustomFields = append(v.CustomFields, row)
	}
	return v
}

// ItemSetResponseData is the resource object of an item set. Its ID is the
// set's name.
type ItemSetResponseData struct {
	ID         string      `json:"id" example:"collection"`
	Type       string      `json:"type" example:"item_sets" enums:"item_sets"`
	Attributes ItemSetView `json:"attributes"`
}

// ItemSetMeta reports what an add or a remove did. An add fills the created
// and added counts, a remove the deleted and kept names.
type ItemSetMeta struct {
	CreatedItemTypes    int `json:"created_item_types" example:"7" format:"int64"`
	CreatedCustomFields int `json:"created_custom_fields" example:"8" format:"int64"`
	// AddedTemplates counts the fields the add put on a type's template,
	// besides those of the fields it created.
	AddedTemplates int `json:"added_templates" example:"0" format:"int64"`
	// DeletedItemTypes and DeletedCustomFields name what a remove deleted.
	DeletedItemTypes    []string `json:"deleted_item_types"`
	DeletedCustomFields []string `json:"deleted_custom_fields"`
	// KeptItemTypes and KeptCustomFields name what the remove request
	// listed but kept: an item started to use it since the confirmation,
	// or it became the group's last type.
	KeptItemTypes    []string `json:"kept_item_types"`
	KeptCustomFields []string `json:"kept_custom_fields"`
}

// ItemSetResponse is the JSON:API envelope of an item set.
type ItemSetResponse struct {
	HTTPStatusCode int                  `json:"-"`
	Data           *ItemSetResponseData `json:"data"`
	Meta           *ItemSetMeta         `json:"meta,omitempty"`
}

// NewItemSetResponse wraps the status of a set.
func NewItemSetResponse(st *services.ItemSetStatus) *ItemSetResponse {
	return &ItemSetResponse{Data: &ItemSetResponseData{
		ID:         string(st.Set),
		Type:       "item_sets",
		Attributes: newItemSetView(st),
	}}
}

// NewItemSetAddResponse wraps the status after an add with its counts.
func NewItemSetAddResponse(res *services.ItemSetAddResult) *ItemSetResponse {
	resp := NewItemSetResponse(res.Status)
	resp.Meta = &ItemSetMeta{
		CreatedItemTypes:    res.CreatedItemTypes,
		CreatedCustomFields: res.CreatedFields,
		AddedTemplates:      res.AddedTemplates,
		DeletedItemTypes:    []string{},
		DeletedCustomFields: []string{},
		KeptItemTypes:       []string{},
		KeptCustomFields:    []string{},
	}
	return resp
}

// NewItemSetRemoveResponse wraps the status after a remove with what it
// deleted and what it kept of what the request listed.
func NewItemSetRemoveResponse(res *services.ItemSetRemoveResult) *ItemSetResponse {
	resp := NewItemSetResponse(res.Status)
	resp.Meta = &ItemSetMeta{
		DeletedItemTypes:    nonNil(res.DeletedItemTypes),
		DeletedCustomFields: nonNil(res.DeletedFields),
		KeptItemTypes:       nonNil(res.KeptItemTypes),
		KeptCustomFields:    nonNil(res.KeptFields),
	}
	return resp
}

// nonNil returns names, or an empty list for nil, so the meta lists are
// arrays in JSON.
func nonNil(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}

var _ render.Binder = (*ItemSetRemoveRequest)(nil)

// ItemSetRemoveRequest is the body of DELETE /groups/{groupID}/collection-set:
// the item types and custom fields the confirmation listed as deleted.
type ItemSetRemoveRequest struct {
	Data *ItemSetRemoveData `json:"data"`
}

type ItemSetRemoveData struct {
	Type       string                   `json:"type" example:"item_sets" enums:"item_sets"`
	Attributes *ItemSetRemoveAttributes `json:"attributes"`
}

type ItemSetRemoveAttributes struct {
	// ItemTypeIDs and CustomFieldIDs name the rows the remove may delete,
	// by ID. It deletes those that are removable when it runs; one that
	// stopped being removable stays and is listed in meta, and an ID that
	// names no type or field of the set is ignored. Either list may be
	// empty.
	ItemTypeIDs    []string `json:"item_type_ids"`
	CustomFieldIDs []string `json:"custom_field_ids"`
}

func (d *ItemSetRemoveData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.Type, validation.Required, validation.In("item_sets")),
		validation.Field(&d.Attributes, validation.Required),
	)
}

func (a *ItemSetRemoveAttributes) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, a,
		validation.Field(&a.ItemTypeIDs, validation.NotNil, validation.Length(0, models.MaxItemTypesPerGroup), validation.Each(validation.Required)),
		validation.Field(&a.CustomFieldIDs, validation.NotNil, validation.Length(0, models.MaxCustomFieldsPerGroup), validation.Each(validation.Required)),
	)
}

func (rr *ItemSetRemoveRequest) Bind(r *http.Request) error {
	if rr.Data == nil {
		return validation.NewError("validation_required", "data is required")
	}
	if err := rr.Data.ValidateWithContext(r.Context()); err != nil {
		return err
	}
	return rr.Data.Attributes.ValidateWithContext(r.Context())
}

func (rd *ItemSetResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

// LocationGroupCreateMeta reports on the extras a create request asked for.
type LocationGroupCreateMeta struct {
	// CollectionSet reports on adding the Collection set, when the request
	// had collection_set.
	CollectionSet *GroupCreateItemSetOutcome `json:"collection_set,omitempty"`
}

// GroupCreateItemSetOutcome is how adding an item set to a new group went.
// The group exists either way. When Added is false, adding the set from the
// group settings finishes it.
type GroupCreateItemSetOutcome struct {
	Added               bool `json:"added" example:"true"`
	CreatedItemTypes    int  `json:"created_item_types" example:"7" format:"int64"`
	CreatedCustomFields int  `json:"created_custom_fields" example:"8" format:"int64"`
}
