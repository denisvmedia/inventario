package services

import (
	"context"
	"errors"
	"slices"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// ItemSetEntryState says how a group holds one item type or custom field of
// an item set.
type ItemSetEntryState string

const (
	// ItemSetEntryMissing: the group has no such row; adding the set
	// creates it.
	ItemSetEntryMissing ItemSetEntryState = "missing"
	// ItemSetEntryAdded: the set created the row. Removing the set deletes
	// it while nothing uses it.
	ItemSetEntryAdded ItemSetEntryState = "added"
	// ItemSetEntryOwn: a member's own row with the name of the set's entry.
	// The set uses it and never deletes it.
	ItemSetEntryOwn ItemSetEntryState = "own"
)

// ItemSetKeepReason says why removing the set keeps a row the group has.
type ItemSetKeepReason string

const (
	// ItemSetKeepNone: removing the set deletes the row.
	ItemSetKeepNone ItemSetKeepReason = ""
	// ItemSetKeepOwn: a member created the row, not the set.
	ItemSetKeepOwn ItemSetKeepReason = "own"
	// ItemSetKeepInUse: items, those in Trash included, have the type or a
	// value of the field.
	ItemSetKeepInUse ItemSetKeepReason = "in_use"
	// ItemSetKeepLastType: the type would be the group's last one, and a
	// group keeps at least one type.
	ItemSetKeepLastType ItemSetKeepReason = "last_type"
)

// ItemSetItemTypeStatus is one item type of a set as the group holds it.
type ItemSetItemTypeStatus struct {
	Entry models.ItemSetItemType
	State ItemSetEntryState
	// ItemType is the group's type that stands for the entry, nil while
	// the entry is missing. It may carry another name: a member may have
	// renamed it.
	ItemType *models.ItemType
	Usage    registry.ItemTypeUsage
	// KeepReason is why removing the set keeps the type, for a type the
	// group has.
	KeepReason ItemSetKeepReason
}

// Removable reports whether removing the set deletes the type.
func (s ItemSetItemTypeStatus) Removable() bool {
	return s.State == ItemSetEntryAdded && s.KeepReason == ItemSetKeepNone
}

// ItemSetFieldStatus is one custom field of a set as the group holds it.
type ItemSetFieldStatus struct {
	Entry models.ItemSetField
	State ItemSetEntryState
	// Field is the group's field that stands for the entry, nil while the
	// entry is missing.
	Field *models.CustomFieldDefinition
	Usage registry.CustomFieldUsage
	// KeepReason is why removing the set keeps the field, for a field the
	// group has.
	KeepReason ItemSetKeepReason
}

// Removable reports whether removing the set deletes the field.
func (s ItemSetFieldStatus) Removable() bool {
	return s.State == ItemSetEntryAdded && s.KeepReason == ItemSetKeepNone
}

// ItemSetStatus is how a group holds an item set: every item type and
// custom field of the set, in the set's order.
type ItemSetStatus struct {
	Set       models.ItemSet
	ItemTypes []ItemSetItemTypeStatus
	Fields    []ItemSetFieldStatus
}

// Complete reports whether the group has every type and field of the set,
// its own or the set's.
func (s *ItemSetStatus) Complete() bool {
	for _, t := range s.ItemTypes {
		if t.State == ItemSetEntryMissing {
			return false
		}
	}
	for _, f := range s.Fields {
		if f.State == ItemSetEntryMissing {
			return false
		}
	}
	return true
}

// ItemSetAddResult is what adding a set did.
type ItemSetAddResult struct {
	CreatedItemTypes int
	CreatedFields    int
	// AddedTemplates counts the fields put on a type's template besides
	// those of the fields the set created.
	AddedTemplates int
	// Status is the set as the group holds it afterwards.
	Status *ItemSetStatus
}

// ItemSetRemoveResult is what removing a set did.
type ItemSetRemoveResult struct {
	// DeletedItemTypes and DeletedFields name the rows removed, by the
	// names they had.
	DeletedItemTypes []string
	DeletedFields    []string
	// Status is the set as the group holds it afterwards: what is left of
	// it is kept, each with its reason.
	Status *ItemSetStatus
}

// ItemSetService adds an item set (#1607) to a group and removes it, through
// registries scoped to that group. Who may do so is the caller's check: the
// HTTP layer allows a group admin.
//
// Adding is idempotent. It creates the types and fields the group does not
// have, reuses a type or field of the same name (ignoring case and runs of
// spaces) that a member made, and puts every field of the set on the
// template of every type of the set. A failure halfway leaves what it
// created, and adding again completes it.
//
// Removing deletes only the types and fields the set created that no item
// uses: no item, in Trash or not, of the type, and no item with a value of
// the field. It is safe to repeat; a later run deletes what has become
// unused since.
type ItemSetService struct {
	def       models.ItemSetDefinition
	itemTypes registry.ItemTypeRegistry
	fields    registry.CustomFieldRegistry
}

// NewItemSetService returns the service for set def in the group the two
// registries are scoped to.
func NewItemSetService(def models.ItemSetDefinition, itemTypes registry.ItemTypeRegistry, fields registry.CustomFieldRegistry) *ItemSetService {
	return &ItemSetService{def: def, itemTypes: itemTypes, fields: fields}
}

// Status returns how the group holds the set.
func (s *ItemSetService) Status(ctx context.Context) (*ItemSetStatus, error) {
	types, err := s.itemTypes.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list item types", err)
	}
	counts, err := s.itemTypes.ItemCounts(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to count items per type", err)
	}
	fields, err := s.fields.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list custom fields", err)
	}
	usage, err := s.fields.Usage(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to count custom field values", err)
	}

	st := &ItemSetStatus{Set: s.def.Set}
	typeRows := s.matchItemTypes(types)
	for i, e := range s.def.ItemTypes {
		ts := ItemSetItemTypeStatus{Entry: e, State: ItemSetEntryMissing}
		if row := typeRows[i]; row != nil {
			ts.ItemType = row
			ts.Usage = counts[row.Slug]
			ts.State = s.stateOf(row.SetEntry)
			ts.KeepReason = keepReason(ts.State, ts.Usage.Total())
		}
		st.ItemTypes = append(st.ItemTypes, ts)
	}
	s.keepLastType(st, len(types))

	fieldRows := s.matchFields(fields)
	for i, e := range s.def.Fields {
		fs := ItemSetFieldStatus{Entry: e, State: ItemSetEntryMissing}
		if row := fieldRows[i]; row != nil {
			fs.Field = row
			fs.Usage = usage[row.ID]
			fs.State = s.stateOf(row.SetEntry)
			fs.KeepReason = keepReason(fs.State, fs.Usage.Total())
		}
		st.Fields = append(st.Fields, fs)
	}
	return st, nil
}

// keepLastType keeps one type when removing the set would delete every type
// the group has, as a group cannot be left without one.
func (*ItemSetService) keepLastType(st *ItemSetStatus, groupTypes int) {
	removable := 0
	last := -1
	for i, t := range st.ItemTypes {
		if t.Removable() {
			removable++
			last = i
		}
	}
	if removable > 0 && removable >= groupTypes {
		st.ItemTypes[last].KeepReason = ItemSetKeepLastType
	}
}

func (s *ItemSetService) stateOf(entry models.ItemSetEntry) ItemSetEntryState {
	if entry != "" && entry.Set() == s.def.Set {
		return ItemSetEntryAdded
	}
	return ItemSetEntryOwn
}

func keepReason(state ItemSetEntryState, used int) ItemSetKeepReason {
	switch {
	case state == ItemSetEntryOwn:
		return ItemSetKeepOwn
	case used > 0:
		return ItemSetKeepInUse
	default:
		return ItemSetKeepNone
	}
}

// matchItemTypes returns, per item type of the set, the group's type that
// stands for it, or nil.
func (s *ItemSetService) matchItemTypes(types []*models.ItemType) []*models.ItemType {
	return matchSetRows(s.def.ItemTypes, types,
		func(e models.ItemSetItemType) (models.ItemSetEntry, string) {
			return s.def.Entry(e.Key), models.ItemTypeNameKey(e.Name)
		},
		func(t *models.ItemType) (models.ItemSetEntry, string) { return t.SetEntry, t.NameKey })
}

// matchFields returns, per custom field of the set, the group's field that
// stands for it, or nil.
func (s *ItemSetService) matchFields(fields []*models.CustomFieldDefinition) []*models.CustomFieldDefinition {
	return matchSetRows(s.def.Fields, fields,
		func(e models.ItemSetField) (models.ItemSetEntry, string) {
			return s.def.Entry(e.Key), models.CustomFieldNameKey(e.Name)
		},
		func(f *models.CustomFieldDefinition) (models.ItemSetEntry, string) { return f.SetEntry, f.NameKey })
}

// matchSetRows returns, per entry of a set, the row that stands for it, or
// the zero R. entryKey and rowKey give the set entry and the name key of an
// entry and of a row. The row the entry created wins, whatever it is called
// now, so a renamed type is still the set's. Otherwise a row with the
// entry's name key that no entry took by its set entry stands for it: a
// member's own row, or one a later version of the set created under a key
// this one lacks.
func matchSetRows[E, R any](entries []E, rows []R, entryKey func(E) (models.ItemSetEntry, string), rowKey func(R) (models.ItemSetEntry, string)) []R {
	out := make([]R, len(entries))
	matched := make([]bool, len(entries))
	taken := make([]bool, len(rows))
	for i, e := range entries {
		entry, _ := entryKey(e)
		for j, r := range rows {
			if rowEntry, _ := rowKey(r); !taken[j] && rowEntry == entry {
				out[i], matched[i], taken[j] = r, true, true
				break
			}
		}
	}
	for i, e := range entries {
		if matched[i] {
			continue
		}
		_, name := entryKey(e)
		for j, r := range rows {
			if _, rowName := rowKey(r); !taken[j] && rowName == name {
				out[i], matched[i], taken[j] = r, true, true
				break
			}
		}
	}
	return out
}

// Add adds the set to the group. On an error, the result counts what was
// created before it.
func (s *ItemSetService) Add(ctx context.Context) (*ItemSetAddResult, error) {
	res := &ItemSetAddResult{}
	st, err := s.Status(ctx)
	if err != nil {
		return res, err
	}

	typeIDs := make([]string, 0, len(st.ItemTypes))
	for _, ts := range st.ItemTypes {
		t := ts.ItemType
		if t == nil {
			var created bool
			if t, created, err = s.createItemType(ctx, ts.Entry); err != nil {
				return res, err
			}
			if created {
				res.CreatedItemTypes++
			}
		}
		typeIDs = append(typeIDs, t.ID)
	}

	for _, fs := range st.Fields {
		f := fs.Field
		if f == nil {
			var created bool
			if f, created, err = s.createField(ctx, fs.Entry, typeIDs); err != nil {
				return res, err
			}
			if created {
				res.CreatedFields++
				continue
			}
		}
		added, err := s.fields.AddTemplates(ctx, f.ID, typeIDs)
		if err != nil {
			return res, errxtrace.Wrap("failed to add the field to the set's types", err, errx.Attrs("custom_field_id", f.ID))
		}
		res.AddedTemplates += added
	}

	if res.Status, err = s.Status(ctx); err != nil {
		return res, err
	}
	return res, nil
}

// createItemType creates the type of entry e. When a type of that name
// appeared meanwhile, it returns that one with created false.
func (s *ItemSetService) createItemType(ctx context.Context, e models.ItemSetItemType) (*models.ItemType, bool, error) {
	t, err := s.itemTypes.Create(ctx, models.ItemType{Name: e.Name, Icon: e.Icon, SetEntry: s.def.Entry(e.Key)})
	if err == nil {
		return t, true, nil
	}
	if errors.Is(err, registry.ErrAlreadyExists) {
		if existing, gerr := s.itemTypes.GetByName(ctx, e.Name); gerr == nil {
			return existing, false, nil
		}
	}
	return nil, false, errxtrace.Wrap("failed to create the set's item type", err, errx.Attrs("name", e.Name))
}

// createField creates the field of entry e on the templates of typeIDs.
// When a field of that name appeared meanwhile, it returns that one with
// created false.
func (s *ItemSetService) createField(ctx context.Context, e models.ItemSetField, typeIDs []string) (*models.CustomFieldDefinition, bool, error) {
	templates := make([]models.CustomFieldTemplate, 0, len(typeIDs))
	for _, id := range typeIDs {
		templates = append(templates, models.CustomFieldTemplate{ItemTypeID: id})
	}
	def := models.CustomFieldDefinition{
		Name:      e.Name,
		Type:      e.Type,
		Options:   slices.Clone(models.ValuerSlice[models.CustomFieldOption](e.Options)),
		SetEntry:  s.def.Entry(e.Key),
		Templates: templates,
	}
	f, err := s.fields.Create(ctx, def)
	if err == nil {
		return f, true, nil
	}
	if errors.Is(err, registry.ErrAlreadyExists) {
		if existing, gerr := s.fields.GetByName(ctx, e.Name); gerr == nil {
			return existing, false, nil
		}
	}
	return nil, false, errxtrace.Wrap("failed to create the set's custom field", err, errx.Attrs("name", e.Name))
}

// Remove deletes the types and fields the set created that nothing uses. A
// row an item started to use meanwhile is kept, as the registries refuse to
// delete it. On an error, the result names what was deleted before it.
func (s *ItemSetService) Remove(ctx context.Context) (*ItemSetRemoveResult, error) {
	res := &ItemSetRemoveResult{}
	st, err := s.Status(ctx)
	if err != nil {
		return res, err
	}

	for _, fs := range st.Fields {
		if !fs.Removable() {
			continue
		}
		err := s.fields.Delete(ctx, fs.Field.ID)
		switch {
		case err == nil:
			res.DeletedFields = append(res.DeletedFields, fs.Field.Name)
		case errors.Is(err, registry.ErrCustomFieldInUse), errors.Is(err, registry.ErrNotFound):
		default:
			return res, errxtrace.Wrap("failed to delete the set's custom field", err, errx.Attrs("custom_field_id", fs.Field.ID))
		}
	}

	for _, ts := range st.ItemTypes {
		if !ts.Removable() {
			continue
		}
		_, err := s.itemTypes.DeleteReassigning(ctx, ts.ItemType.ID, "")
		switch {
		case err == nil:
			res.DeletedItemTypes = append(res.DeletedItemTypes, ts.ItemType.DisplayName())
		case errors.Is(err, registry.ErrItemTypeInUse), errors.Is(err, registry.ErrLastItemType), errors.Is(err, registry.ErrNotFound):
		default:
			return res, errxtrace.Wrap("failed to delete the set's item type", err, errx.Attrs("item_type_id", ts.ItemType.ID))
		}
	}

	if res.Status, err = s.Status(ctx); err != nil {
		return res, err
	}
	return res, nil
}
