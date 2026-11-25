package registry

import (
	"bytes"
	"maps"
	"slices"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
)

// The reasons an item write's custom field value is refused, reported per
// field in a CustomFieldValueError. The value errors of models
// (CustomFieldDefinition.ParseValue) are reported the same way.
var (
	ErrCustomFieldUnknown   = validation.NewError("custom_field_unknown", "is not a custom field of this group")
	ErrCustomFieldLinkSelf  = validation.NewError("custom_field_link_to_self", "an item cannot link to itself")
	ErrCustomFieldLinkGone  = validation.NewError("custom_field_link_not_found", "is not an item of this group")
	ErrCustomFieldLinkTrash = validation.NewError("custom_field_link_trashed", "the item is in Trash")
)

// CustomFieldWritePlan is what an item write does to the item's stored
// custom field values: the fields whose value it deletes and the values it
// sets. A value equal to the stored one is in neither.
type CustomFieldWritePlan struct {
	Deletes []string
	Sets    map[string]models.CustomFieldData
}

// LinkedItemState says whether an item a link names exists in the group and
// whether it is in Trash.
type LinkedItemState struct {
	Exists  bool
	Trashed bool
}

// PlanCustomFieldWrite checks the custom field values of a write to the
// item selfID and returns what to change. defs are the group's fields the
// input names, stored the item's values (canonical API form), and
// lookupItem reads the state of an item a new link names; it is called only
// for a link that differs from the stored one, so a kept link to an item in
// Trash stays. Every refused value is reported in one
// *CustomFieldValueError; an error from lookupItem is returned as it is.
//
// It is the one statement of the write rules both backends follow.
func PlanCustomFieldWrite(
	selfID string,
	defs map[string]*models.CustomFieldDefinition,
	stored, input models.CustomFieldValues,
	lookupItem func(id string) (LinkedItemState, error),
) (CustomFieldWritePlan, error) {
	plan := CustomFieldWritePlan{Sets: map[string]models.CustomFieldData{}}
	fieldErrs := map[string]error{}
	for _, id := range slices.Sorted(maps.Keys(input)) {
		def, ok := defs[id]
		if !ok {
			// Clearing a field that no longer exists asks for what is
			// already true: its values went with it.
			if !models.IsNullJSON(input[id]) {
				fieldErrs[id] = ErrCustomFieldUnknown
			}
			continue
		}
		old, had := stored[id]
		if models.IsNullJSON(input[id]) {
			if had {
				plan.Deletes = append(plan.Deletes, id)
			}
			continue
		}
		data, err := def.ParseValue(input[id])
		if err != nil {
			fieldErrs[id] = err
			continue
		}
		if had && bytes.Equal(old, data.JSON(def.Type)) {
			continue
		}
		if def.Type == models.CustomFieldTypeItemLink {
			refusal, err := checkLink(selfID, *data.ItemID, lookupItem)
			if err != nil {
				return CustomFieldWritePlan{}, err
			}
			if refusal != nil {
				fieldErrs[id] = refusal
				continue
			}
		}
		plan.Sets[id] = data
	}
	if len(fieldErrs) > 0 {
		return CustomFieldWritePlan{}, &CustomFieldValueError{Fields: fieldErrs}
	}
	return plan, nil
}

func checkLink(selfID, targetID string, lookupItem func(string) (LinkedItemState, error)) (refusal, err error) {
	if targetID == selfID {
		return ErrCustomFieldLinkSelf, nil
	}
	state, err := lookupItem(targetID)
	switch {
	case err != nil:
		return nil, err
	case !state.Exists:
		return ErrCustomFieldLinkGone, nil
	case state.Trashed:
		return ErrCustomFieldLinkTrash, nil
	}
	return nil, nil
}

// ApplyCustomFieldWritePlan returns stored with plan applied, as a new map
// (nil when no value is left); stored itself is not changed. defs give the
// type each set value is written in.
func ApplyCustomFieldWritePlan(stored models.CustomFieldValues, plan CustomFieldWritePlan, defs map[string]*models.CustomFieldDefinition) models.CustomFieldValues {
	out := stored.Clone()
	if out == nil {
		out = models.CustomFieldValues{}
	}
	for _, id := range plan.Deletes {
		delete(out, id)
	}
	for id, data := range plan.Sets {
		out[id] = data.JSON(defs[id].Type)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
