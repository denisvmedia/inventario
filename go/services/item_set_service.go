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
	// ItemSetEntryTaken: the group has no row for the entry, and the row
	// that stands for another entry of the set has the entry's name, as a
	// member renamed it. Adding cannot create the entry under that name and
	// leaves it out.
	ItemSetEntryTaken ItemSetEntryState = "taken"
)

// ErrItemSetChanged reports an add that found the group's item types or
// custom fields changed under it twice, as a remove or a member deleted
// one of the rows it was working with. Adding again starts afresh.
var ErrItemSetChanged = errx.NewSentinel("the group's item types or custom fields changed while the set was being added")

// errItemSetIncomplete reports an add pass whose own work is done while the
// status read after it still lacks a row or a link, as a member deleted one
// the pass had just written.
var errItemSetIncomplete = errx.NewSentinel("the set is not complete after adding it")

// ItemSetKeepReason says why removing the set keeps a row the group has.
type ItemSetKeepReason string

const (
	// ItemSetKeepNone: removing the set deletes the row.
	ItemSetKeepNone ItemSetKeepReason = ""
	// ItemSetKeepOwn: a member created the row, not the set.
	ItemSetKeepOwn ItemSetKeepReason = "own"
	// ItemSetKeepInUse: items, those in Trash included, or open drafts have
	// the type or a value of the field.
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
	// the entry is missing or taken. It may carry another name: a member
	// may have renamed it.
	ItemType *models.ItemType
	// TakenBy is the key of the entry whose type has this entry's name, for
	// a taken entry.
	TakenBy string
	Usage   registry.ItemTypeUsage
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
	// entry is missing or taken.
	Field *models.CustomFieldDefinition
	// TakenBy is the key of the entry whose field has this entry's name,
	// for a taken entry.
	TakenBy string
	// MissingTemplates counts the group's types of the set that do not
	// offer the field: adding the set puts it on their templates.
	MissingTemplates int
	Usage            registry.CustomFieldUsage
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

// Complete reports whether adding the set has nothing left to do: the group
// has every type and field of the set, its own or the set's, but for those
// whose name another entry's row holds, and every such field is offered on
// every such type.
func (s *ItemSetStatus) Complete() bool {
	for _, t := range s.ItemTypes {
		if t.State == ItemSetEntryMissing {
			return false
		}
	}
	for _, f := range s.Fields {
		if f.State == ItemSetEntryMissing || f.MissingTemplates > 0 {
			return false
		}
	}
	return true
}

// typeIDs returns the IDs of the group's types that stand for the set's
// entries, each once.
func (s *ItemSetStatus) typeIDs() []string {
	ids := make([]string, 0, len(s.ItemTypes))
	for _, t := range s.ItemTypes {
		if t.ItemType != nil && !slices.Contains(ids, t.ItemType.ID) {
			ids = append(ids, t.ItemType.ID)
		}
	}
	return ids
}

// Removal returns what a confirmation built from s lists as deleted: the
// types and fields removing the set deletes.
func (s *ItemSetStatus) Removal() ItemSetRemoval {
	sel := ItemSetRemoval{ItemTypeIDs: []string{}, FieldIDs: []string{}}
	for _, t := range s.ItemTypes {
		if t.Removable() {
			sel.ItemTypeIDs = append(sel.ItemTypeIDs, t.ItemType.ID)
		}
	}
	for _, f := range s.Fields {
		if f.Removable() {
			sel.FieldIDs = append(sel.FieldIDs, f.Field.ID)
		}
	}
	return sel
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

// ItemSetRemoval names the rows a remove may delete, by ID: those the
// confirmation listed as deleted. The remove deletes those among them that
// are removable when it runs, and no other row.
type ItemSetRemoval struct {
	ItemTypeIDs []string
	FieldIDs    []string
}

// ItemSetRemoveResult is what removing a set did.
type ItemSetRemoveResult struct {
	// DeletedItemTypes and DeletedFields name the rows removed, by the
	// names they had.
	DeletedItemTypes []string
	DeletedFields    []string
	// KeptItemTypes and KeptFields name the rows the removal listed that
	// stayed: an item or a draft started to use one since the confirmation,
	// or it became the group's last type.
	KeptItemTypes []string
	KeptFields    []string
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
// Removing deletes only the types and fields the set created that nothing
// uses: no item, in Trash or not, and no open draft of the type, and no item
// or open draft with a value of the field, and never the group's last type. It deletes no more than the
// caller names, so what a confirmation listed bounds it. It is safe to
// repeat; a later run deletes what has become unused since.
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
	typeRows, typesTakenBy := s.matchItemTypes(types)
	for i, e := range s.def.ItemTypes {
		ts := ItemSetItemTypeStatus{Entry: e, State: ItemSetEntryMissing}
		switch row := typeRows[i]; {
		case row != nil:
			ts.ItemType = row
			ts.Usage = counts[row.Slug]
			ts.State = s.stateOf(row.SetEntry)
			ts.KeepReason = keepReason(ts.State, ts.Usage.InUse())
		case typesTakenBy[i] >= 0:
			ts.State = ItemSetEntryTaken
			ts.TakenBy = s.def.ItemTypes[typesTakenBy[i]].Key
		}
		st.ItemTypes = append(st.ItemTypes, ts)
	}
	s.keepLastType(st, len(types))

	fieldRows, fieldsTakenBy := s.matchFields(fields)
	for i, e := range s.def.Fields {
		fs := ItemSetFieldStatus{Entry: e, State: ItemSetEntryMissing}
		switch row := fieldRows[i]; {
		case row != nil:
			fs.Field = row
			fs.Usage = usage[row.ID]
			fs.State = s.stateOf(row.SetEntry)
			fs.KeepReason = keepReason(fs.State, fs.Usage.InUse())
		case fieldsTakenBy[i] >= 0:
			fs.State = ItemSetEntryTaken
			fs.TakenBy = s.def.Fields[fieldsTakenBy[i]].Key
		}
		st.Fields = append(st.Fields, fs)
	}

	setTypes := st.typeIDs()
	for i := range st.Fields {
		if f := st.Fields[i].Field; f != nil {
			st.Fields[i].MissingTemplates = missingTemplates(f.Templates, setTypes)
		}
	}
	return st, nil
}

// missingTemplates counts the types of typeIDs that templates do not name.
func missingTemplates(templates []models.CustomFieldTemplate, typeIDs []string) int {
	missing := 0
	for _, id := range typeIDs {
		if !slices.ContainsFunc(templates, func(t models.CustomFieldTemplate) bool { return t.ItemTypeID == id }) {
			missing++
		}
	}
	return missing
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

func keepReason(state ItemSetEntryState, inUse bool) ItemSetKeepReason {
	switch {
	case state == ItemSetEntryOwn:
		return ItemSetKeepOwn
	case inUse:
		return ItemSetKeepInUse
	default:
		return ItemSetKeepNone
	}
}

// matchItemTypes returns, per item type of the set, the group's type that
// stands for it, or nil, and the index of the entry whose type holds its
// name, or -1.
func (s *ItemSetService) matchItemTypes(types []*models.ItemType) ([]*models.ItemType, []int) {
	return matchSetRows(s.def.ItemTypes, types,
		func(e models.ItemSetItemType) (models.ItemSetEntry, string) {
			return s.def.Entry(e.Key), models.ItemTypeNameKey(e.Name)
		},
		func(t *models.ItemType) (models.ItemSetEntry, string) { return t.SetEntry, t.NameKey })
}

// matchFields returns, per custom field of the set, the group's field that
// stands for it, or nil, and the index of the entry whose field holds its
// name, or -1.
func (s *ItemSetService) matchFields(fields []*models.CustomFieldDefinition) ([]*models.CustomFieldDefinition, []int) {
	return matchSetRows(s.def.Fields, fields,
		func(e models.ItemSetField) (models.ItemSetEntry, string) {
			return s.def.Entry(e.Key), models.CustomFieldNameKey(e.Name)
		},
		func(f *models.CustomFieldDefinition) (models.ItemSetEntry, string) { return f.SetEntry, f.NameKey })
}

// matchSetRows returns, per entry of a set, the row that stands for it, or
// the zero R, and the index of the entry whose row holds its name, or -1.
// entryKey and rowKey give the set entry and the name key of an entry and
// of a row. The row the entry created wins, whatever it is called now, so a
// renamed type is still the set's. Otherwise a row with the entry's name key
// that no entry took by its set entry stands for it: a member's own row, or
// one a later version of the set created under a key this one lacks. A row
// stands for one entry at most; when the row with an entry's name stands
// for another entry, a member renamed that one, and the entry is taken.
func matchSetRows[E, R any](entries []E, rows []R, entryKey func(E) (models.ItemSetEntry, string), rowKey func(R) (models.ItemSetEntry, string)) ([]R, []int) {
	out := make([]R, len(entries))
	takenBy := make([]int, len(entries))
	matched := make([]bool, len(entries))
	owner := make([]int, len(rows))
	for j := range owner {
		owner[j] = -1
	}
	for i, e := range entries {
		takenBy[i] = -1
		entry, _ := entryKey(e)
		for j, r := range rows {
			if rowEntry, _ := rowKey(r); owner[j] < 0 && rowEntry == entry {
				out[i], matched[i], owner[j] = r, true, i
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
			if _, rowName := rowKey(r); rowName != name {
				continue
			}
			if owner[j] < 0 {
				out[i], matched[i], owner[j] = r, true, i
			} else {
				takenBy[i] = owner[j]
			}
			break
		}
	}
	return out, takenBy
}

// Add adds the set to the group. It succeeds only when the status read
// after it is complete. On an error, the result counts what was created
// before it.
//
// A type or field the add works with can go away under it, as a remove or
// a member deletes it. The registries then report it gone, or refuse a
// template that names it, or the status read afterwards lacks it; the add
// runs once more from a fresh status, keeping its counts, and fails with
// ErrItemSetChanged when that happens again.
func (s *ItemSetService) Add(ctx context.Context) (*ItemSetAddResult, error) {
	res := &ItemSetAddResult{}
	err := s.addAndCheck(ctx, res)
	if changedUnder(err) {
		err = s.addAndCheck(ctx, res)
		if changedUnder(err) {
			err = errxtrace.Classify(err, ErrItemSetChanged)
		}
	}
	return res, err
}

// addAndCheck runs one pass of Add and reads the status it leaves into res,
// failing with errItemSetIncomplete when that status is not complete.
func (s *ItemSetService) addAndCheck(ctx context.Context, res *ItemSetAddResult) error {
	if err := s.add(ctx, res); err != nil {
		return err
	}
	st, err := s.Status(ctx)
	if err != nil {
		return err
	}
	res.Status = st
	if !st.Complete() {
		return errxtrace.Wrap("failed to complete the set", errItemSetIncomplete)
	}
	return nil
}

// changedUnder reports whether err is a registry refusing a row that went
// away during an add, or a status that lost one after it.
func changedUnder(err error) bool {
	return errors.Is(err, registry.ErrNotFound) || errors.Is(err, registry.ErrInvalidInput) ||
		errors.Is(err, errItemSetIncomplete)
}

// add runs one pass of Add, counting into res.
func (s *ItemSetService) add(ctx context.Context, res *ItemSetAddResult) error {
	st, err := s.Status(ctx)
	if err != nil {
		return err
	}

	// typeIDs lists each type once: the name a missing entry is created
	// under can turn out to belong to a row another entry stands for.
	typeIDs := make([]string, 0, len(st.ItemTypes))
	for _, ts := range st.ItemTypes {
		t := ts.ItemType
		switch ts.State {
		case ItemSetEntryTaken:
			continue
		case ItemSetEntryMissing:
			var created bool
			if t, created, err = s.createItemType(ctx, ts.Entry); err != nil {
				return err
			}
			if created {
				res.CreatedItemTypes++
			}
		}
		if !slices.Contains(typeIDs, t.ID) {
			typeIDs = append(typeIDs, t.ID)
		}
	}

	for _, fs := range st.Fields {
		f := fs.Field
		switch fs.State {
		case ItemSetEntryTaken:
			continue
		case ItemSetEntryMissing:
			var created bool
			if f, created, err = s.createField(ctx, fs.Entry, typeIDs); err != nil {
				return err
			}
			if created {
				res.CreatedFields++
				continue
			}
		}
		added, err := s.fields.AddTemplates(ctx, f.ID, typeIDs)
		if err != nil {
			return errxtrace.Wrap("failed to add the field to the set's types", err, errx.Attrs("custom_field_id", f.ID))
		}
		res.AddedTemplates += added
	}
	return nil
}

// createItemType creates the type of entry e. When a type of that name
// appeared meanwhile, it returns that one with created false.
func (s *ItemSetService) createItemType(ctx context.Context, e models.ItemSetItemType) (*models.ItemType, bool, error) {
	t, err := s.itemTypes.Create(ctx, models.ItemType{Name: e.Name, Icon: e.Icon, SetEntry: s.def.Entry(e.Key)})
	if err == nil {
		return t, true, nil
	}
	if errors.Is(err, registry.ErrAlreadyExists) {
		// A lookup that finds nothing means the type went away again: its
		// ErrNotFound makes Add run once more.
		existing, gerr := s.itemTypes.GetByName(ctx, e.Name)
		if gerr != nil {
			return nil, false, errxtrace.Wrap("failed to get the item type of the set's name", gerr, errx.Attrs("name", e.Name))
		}
		return existing, false, nil
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
		// As for a type: a field that went away again makes Add run once
		// more.
		existing, gerr := s.fields.GetByName(ctx, e.Name)
		if gerr != nil {
			return nil, false, errxtrace.Wrap("failed to get the custom field of the set's name", gerr, errx.Attrs("name", e.Name))
		}
		return existing, false, nil
	}
	return nil, false, errxtrace.Wrap("failed to create the set's custom field", err, errx.Attrs("name", e.Name))
}

// Remove deletes the types and fields named in sel that the set created and
// nothing uses. A named row that is not removable when it runs stays and is
// reported as kept, and a row an item or a draft started to use meanwhile
// stays too, as the registries refuse to delete it. An ID that names no row of the set is
// ignored. On an error, the result names what was deleted before it.
func (s *ItemSetService) Remove(ctx context.Context, sel ItemSetRemoval) (*ItemSetRemoveResult, error) {
	res := &ItemSetRemoveResult{}
	st, err := s.Status(ctx)
	if err != nil {
		return res, err
	}

	for _, fs := range st.Fields {
		if fs.Field == nil || !slices.Contains(sel.FieldIDs, fs.Field.ID) {
			continue
		}
		if !fs.Removable() {
			res.KeptFields = append(res.KeptFields, fs.Field.Name)
			continue
		}
		err := s.fields.Delete(ctx, fs.Field.ID)
		switch {
		case err == nil:
			res.DeletedFields = append(res.DeletedFields, fs.Field.Name)
		case errors.Is(err, registry.ErrCustomFieldInUse):
			res.KeptFields = append(res.KeptFields, fs.Field.Name)
		case errors.Is(err, registry.ErrNotFound):
		default:
			return res, errxtrace.Wrap("failed to delete the set's custom field", err, errx.Attrs("custom_field_id", fs.Field.ID))
		}
	}

	for _, ts := range st.ItemTypes {
		if ts.ItemType == nil || !slices.Contains(sel.ItemTypeIDs, ts.ItemType.ID) {
			continue
		}
		// The status keeps a last type counting every removable type, not
		// the selected ones; the registry refuses the delete that would
		// leave the group without a type, so that check is left to it.
		if !ts.Removable() && ts.KeepReason != ItemSetKeepLastType {
			res.KeptItemTypes = append(res.KeptItemTypes, ts.ItemType.DisplayName())
			continue
		}
		_, err := s.itemTypes.DeleteReassigning(ctx, ts.ItemType.ID, "")
		switch {
		case err == nil:
			res.DeletedItemTypes = append(res.DeletedItemTypes, ts.ItemType.DisplayName())
		case errors.Is(err, registry.ErrItemTypeInUse), errors.Is(err, registry.ErrLastItemType):
			res.KeptItemTypes = append(res.KeptItemTypes, ts.ItemType.DisplayName())
		case errors.Is(err, registry.ErrNotFound):
		default:
			return res, errxtrace.Wrap("failed to delete the set's item type", err, errx.Attrs("item_type_id", ts.ItemType.ID))
		}
	}

	if res.Status, err = s.Status(ctx); err != nil {
		return res, err
	}
	return res, nil
}
