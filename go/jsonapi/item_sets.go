package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"

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
	// creates it, added when the set created the type, and own when it is
	// a member's type of the same name, which the set uses and never
	// deletes.
	State string `json:"state" example:"added" enums:"missing,added,own"`
	// ItemTypeID is the ID of the group's type; absent while missing.
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
	// State is missing, added or own, as for an item type.
	State string `json:"state" example:"added" enums:"missing,added,own"`
	// CustomFieldID is the ID of the group's field; absent while missing.
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
	// Complete is true when the group has every type and field of the set.
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
			Icon:  t.Entry.Icon,
			State: string(t.State),
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
// and added counts, a remove the deleted names.
type ItemSetMeta struct {
	CreatedItemTypes    int `json:"created_item_types" example:"7" format:"int64"`
	CreatedCustomFields int `json:"created_custom_fields" example:"8" format:"int64"`
	// AddedTemplates counts the fields the add put on a type's template,
	// besides those of the fields it created.
	AddedTemplates int `json:"added_templates" example:"0" format:"int64"`
	// DeletedItemTypes and DeletedCustomFields name what a remove deleted.
	DeletedItemTypes    []string `json:"deleted_item_types"`
	DeletedCustomFields []string `json:"deleted_custom_fields"`
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
	}
	return resp
}

// NewItemSetRemoveResponse wraps the status after a remove with what it
// deleted.
func NewItemSetRemoveResponse(res *services.ItemSetRemoveResult) *ItemSetResponse {
	resp := NewItemSetResponse(res.Status)
	meta := &ItemSetMeta{DeletedItemTypes: res.DeletedItemTypes, DeletedCustomFields: res.DeletedFields}
	if meta.DeletedItemTypes == nil {
		meta.DeletedItemTypes = []string{}
	}
	if meta.DeletedCustomFields == nil {
		meta.DeletedCustomFields = []string{}
	}
	resp.Meta = meta
	return resp
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
