//go:build !legacy_xml_backup

package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// restoredCustomField is the destination field of an archived custom field
// (#2803) and the destination ID of each archived select option.
type restoredCustomField struct {
	def     *models.CustomFieldDefinition
	options map[string]string
}

// inbPendingLink is an item link value that can be set only once every item
// of the archive is restored, because the item it names may come later in
// the walk.
type inbPendingLink struct {
	commodityUUID string
	fieldID       string
	targetUUID    string
}

// restoreCustomFields maps the archived custom fields onto the destination
// group's before the first item names one. A field is matched by its UUID
// first, so a field renamed since the backup is still the same field, and
// then by name, the way the group keeps names unique. Every UUID match is
// made before the first name match, and a field matched by UUID is not
// matched by name too: when the group renamed a field to the name of
// another archived field, that other field is counted as an error and its
// values are left out, so two archived fields never share one destination.
// A match of the same type is reused, its templates left as they are, and
// gets the archived options it lacks (see mergeCustomFieldOptions). A name
// match of another type cannot hold the archived values, so the field is
// counted as an error and its values are left out. A field the group lacks
// is created with its archived options and templates. A dry run writes
// nothing and maps nothing, but counts the fields a restore would refuse.
func (w *inbWalker) restoreCustomFields(archived []types.INBCustomField) error {
	w.customFields = map[string]*restoredCustomField{}
	if len(archived) == 0 || w.proc.factorySet.CustomFieldRegistryFactory == nil {
		return nil
	}
	reg, err := w.proc.factorySet.CustomFieldRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create custom field registry", err)
	}
	list, err := reg.List(w.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list custom fields", err)
	}
	index := newCustomFieldIndex(list)
	index.claim(archived)
	for _, a := range archived {
		if a.ID == "" {
			continue
		}
		restored, err := w.restoreCustomField(reg, index, a)
		if refusal, ok := errors.AsType[customFieldRefusal](err); ok {
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("custom field %s: %v", a.ID, refusal.err))
			continue
		}
		if err != nil {
			return errxtrace.Wrap("failed to restore custom field", err, errx.Attrs("custom_field_id", a.ID))
		}
		if restored != nil {
			w.customFields[a.ID] = restored
		}
	}
	return nil
}

// customFieldIndex is the destination group's fields by UUID and by name
// key, kept current as the restore creates and updates fields.
type customFieldIndex struct {
	byUUID map[string]*models.CustomFieldDefinition
	byName map[string]*models.CustomFieldDefinition
	// claimed maps the ID of a destination field an archived field matches
	// by UUID to that archived field's UUID.
	claimed map[string]string
	// count is how many fields the group has, counting the ones a dry run
	// would create.
	count int
}

func newCustomFieldIndex(list []*models.CustomFieldDefinition) *customFieldIndex {
	index := &customFieldIndex{
		byUUID:  make(map[string]*models.CustomFieldDefinition, len(list)),
		byName:  make(map[string]*models.CustomFieldDefinition, len(list)),
		claimed: map[string]string{},
	}
	for _, d := range list {
		index.put(d)
	}
	return index
}

func (x *customFieldIndex) put(d *models.CustomFieldDefinition) {
	if _, ok := x.byUUID[d.UUID]; !ok {
		x.count++
	}
	x.byUUID[d.UUID] = d
	x.byName[d.NameKey] = d
}

// claim records the destination field each archived field matches by UUID
// and type, before any field is matched by name.
func (x *customFieldIndex) claim(archived []types.INBCustomField) {
	for _, a := range archived {
		if d := x.byUUID[a.ID]; d != nil && string(d.Type) == a.Type {
			x.claimed[d.ID] = a.ID
		}
	}
}

// match returns the destination field of a: the field with its UUID and
// type, else the field with its name, whatever its type.
func (x *customFieldIndex) match(a types.INBCustomField) *models.CustomFieldDefinition {
	if d := x.byUUID[a.ID]; d != nil && string(d.Type) == a.Type {
		return d
	}
	return x.byName[models.CustomFieldNameKey(a.Name)]
}

// customFieldRefusal is an archived field the restore reports and moves
// past, as opposed to an error that stops the restore.
type customFieldRefusal struct{ err error }

func (r customFieldRefusal) Error() string { return r.err.Error() }

// restoreCustomField returns the destination of one archived field, nil in
// a dry run, or a customFieldRefusal.
func (w *inbWalker) restoreCustomField(reg registry.CustomFieldRegistry, index *customFieldIndex, a types.INBCustomField) (*restoredCustomField, error) {
	if existing := index.match(a); existing != nil {
		if owner, ok := index.claimed[existing.ID]; ok && owner != a.ID {
			return nil, customFieldRefusal{fmt.Errorf("the group's field %q is the backup's field %s, renamed since", existing.Name, owner)}
		}
		if string(existing.Type) != a.Type {
			return nil, customFieldRefusal{fmt.Errorf("the group has a field named %q of type %s", existing.Name, existing.Type)}
		}
		return w.mergeCustomFieldOptions(reg, index, existing, a)
	}

	def := models.CustomFieldDefinition{Name: a.Name, Type: models.CustomFieldType(a.Type)}
	if entry := models.ItemSetEntry(a.SetEntry); entry.IsValid() {
		def.SetEntry = entry
	}
	for _, o := range a.Options {
		def.Options = append(def.Options, models.CustomFieldOption{ID: o.ID, Label: o.Label})
	}
	if w.options.DryRun {
		def.Normalize()
		if verr := def.ValidateFields(w.ctx); verr != nil {
			return nil, customFieldRefusal{verr}
		}
		if index.count >= models.MaxCustomFieldsPerGroup {
			return nil, customFieldRefusal{registry.ErrResourceLimitExceeded}
		}
		index.count++
		return nil, nil
	}
	templates, err := w.restoreCustomFieldTemplates(a.Templates)
	if err != nil {
		return nil, err
	}
	def.Templates = templates
	def.Normalize()
	if verr := def.ValidateFields(w.ctx); verr != nil {
		return nil, customFieldRefusal{verr}
	}
	created, err := reg.Create(w.ctx, def)
	switch {
	case errors.Is(err, registry.ErrResourceLimitExceeded), errors.Is(err, registry.ErrInvalidInput), errors.Is(err, registry.ErrAlreadyExists):
		return nil, customFieldRefusal{err}
	case err != nil:
		return nil, err
	}
	index.put(created)
	return newRestoredCustomField(created, a.Options), nil
}

// mergeCustomFieldOptions adds to an existing select field the archived
// options it has neither the ID nor the label of, so the archived values
// that hold them restore. An option keeps its ID across a backup and a
// restore, so an option relabeled since the backup is still the same
// option. A dry run returns nil once it has counted the options the field
// could not take.
func (w *inbWalker) mergeCustomFieldOptions(reg registry.CustomFieldRegistry, index *customFieldIndex, existing *models.CustomFieldDefinition, a types.INBCustomField) (*restoredCustomField, error) {
	if existing.Type != models.CustomFieldTypeSelect {
		if w.options.DryRun {
			return nil, nil
		}
		return newRestoredCustomField(existing, nil), nil
	}
	labels := make(map[string]bool, len(existing.Options))
	ids := make(map[string]bool, len(existing.Options))
	for _, o := range existing.Options {
		labels[models.CustomFieldNameKey(o.Label)] = true
		ids[o.ID] = true
	}
	merged := *existing
	merged.Options = append(models.ValuerSlice[models.CustomFieldOption](nil), existing.Options...)
	merged.Templates = nil
	added := false
	for _, o := range a.Options {
		key := models.CustomFieldNameKey(o.Label)
		if ids[o.ID] || labels[key] {
			continue
		}
		labels[key] = true
		ids[o.ID] = true
		merged.Options = append(merged.Options, models.CustomFieldOption{ID: o.ID, Label: o.Label})
		added = true
	}
	if added {
		merged.Normalize()
		if verr := merged.ValidateFields(w.ctx); verr != nil {
			// Too many options: keep the field as it is and restore the
			// values whose option it has.
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("custom field %s: %v", a.ID, verr))
			added = false
		}
	}
	switch {
	case w.options.DryRun:
		return nil, nil
	case !added:
		return newRestoredCustomField(existing, a.Options), nil
	}
	updated, err := reg.Update(w.ctx, merged)
	if err != nil {
		return nil, err
	}
	index.put(updated)
	return newRestoredCustomField(updated, a.Options), nil
}

// restoreCustomFieldTemplates resolves the item type slugs of archived
// template entries to the destination group's item types.
func (w *inbWalker) restoreCustomFieldTemplates(archived []types.INBCustomFieldTemplate) ([]models.CustomFieldTemplate, error) {
	if len(archived) == 0 {
		return nil, nil
	}
	resolver, err := w.proc.itemTypeResolver(w.ctx, w.options)
	if err != nil {
		return nil, err
	}
	out := make([]models.CustomFieldTemplate, 0, len(archived))
	for _, t := range archived {
		id, err := resolver.typeID(w.ctx, models.CommodityType(t.ItemType), w.stats)
		if err != nil {
			return nil, err
		}
		if id == "" {
			continue
		}
		out = append(out, models.CustomFieldTemplate{ItemTypeID: id, DefaultValue: models.JSONValue(t.DefaultValue)})
	}
	return out, nil
}

// newRestoredCustomField maps each archived option to the destination
// option with its ID, else to the one with its label.
func newRestoredCustomField(def *models.CustomFieldDefinition, archived []types.INBCustomFieldOption) *restoredCustomField {
	ids := make(map[string]bool, len(def.Options))
	byLabel := make(map[string]string, len(def.Options))
	for _, o := range def.Options {
		ids[o.ID] = true
		byLabel[models.CustomFieldNameKey(o.Label)] = o.ID
	}
	r := &restoredCustomField{def: def, options: make(map[string]string, len(archived))}
	for _, o := range archived {
		switch {
		case ids[o.ID]:
			r.options[o.ID] = o.ID
		case byLabel[models.CustomFieldNameKey(o.Label)] != "":
			r.options[o.ID] = byLabel[models.CustomFieldNameKey(o.Label)]
		}
	}
	return r
}

// restoredCustomFieldValues maps an archived item's custom field values onto
// the destination fields. A value of a field the restore did not map, of an
// option the destination field lacks, or that the destination field refuses
// is left out and counted as an error, so the item restores without it. An
// item link is returned on its own, to be set once every item is restored.
func (w *inbWalker) restoredCustomFieldValues(c *types.INBCommodity) (models.CustomFieldValues, []inbPendingLink) {
	if len(c.CustomFields) == 0 || len(w.customFields) == 0 {
		return nil, nil
	}
	out := models.CustomFieldValues{}
	var links []inbPendingLink
	for archivedID, raw := range c.CustomFields {
		field := w.customFields[archivedID]
		if field == nil || models.IsNullJSON(raw) {
			continue
		}
		value, err := field.value(raw)
		if err != nil {
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("commodity %s: custom field %s: %v", c.ID, archivedID, err))
			continue
		}
		if field.def.Type == models.CustomFieldTypeItemLink {
			var target string
			if json.Unmarshal(value, &target) == nil && target != "" {
				links = append(links, inbPendingLink{commodityUUID: c.ID, fieldID: field.def.ID, targetUUID: target})
			}
			continue
		}
		out[field.def.ID] = value
	}
	if len(out) == 0 {
		return nil, links
	}
	return out, links
}

// restoreDraftCustomFields maps the entries archived draft values hold under
// "custom_fields" onto the destination fields, as an item's values are
// mapped: a select option onto the destination option, a link onto the item
// it names, one this restore created (the drafts member follows every item
// member) or one the group already had. The entries stay as the form kept
// them otherwise; the promotion checks them. One that maps to nothing is
// left out, like the draft's other references, so a draft never names a
// field, an option or an item outside the group.
func (w *inbWalker) restoreDraftCustomFields(values models.CommodityDraftValues) {
	raw, ok := values["custom_fields"]
	if !ok {
		return
	}
	entries, _ := raw.(map[string]any)
	out := make(map[string]any, len(entries))
	for archivedID, entry := range entries {
		field := w.customFields[archivedID]
		if field == nil || entry == nil {
			continue
		}
		switch field.def.Type {
		case models.CustomFieldTypeSelect:
			entry = models.MapDraftCustomFieldReference(entry, func(id string) string { return field.options[id] })
		case models.CustomFieldTypeItemLink:
			entry = models.MapDraftCustomFieldReference(entry, w.commodityDBID)
		}
		if entry == nil {
			continue
		}
		out[field.def.ID] = entry
	}
	if len(out) == 0 {
		delete(values, "custom_fields")
		return
	}
	values["custom_fields"] = out
}

// value returns raw as the destination field stores it: a select value
// names the destination option.
func (f *restoredCustomField) value(raw json.RawMessage) (json.RawMessage, error) {
	if f.def.Type == models.CustomFieldTypeSelect {
		var id string
		if err := json.Unmarshal(raw, &id); err != nil {
			return nil, err
		}
		dest, ok := f.options[id]
		if !ok {
			return nil, errors.New("the field has no such option")
		}
		raw, _ = json.Marshal(dest)
	}
	data, err := f.def.ParseValue(raw)
	if err != nil {
		return nil, err
	}
	return data.JSON(f.def.Type), nil
}

// applyPendingLinks sets the item link values once every item of the archive
// is restored. A link is left out and counted as an error when it names an
// item neither restored nor in the group outside Trash, when the item that
// holds it cannot be read, or when the destination refuses it. A link to an
// item in the group's Trash that the item already holds stays as it is and
// is not counted.
func (w *inbWalker) applyPendingLinks() error {
	if len(w.pendingLinks) == 0 {
		return nil
	}
	comReg, err := w.proc.factorySet.CommodityRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user commodity registry for item links", err)
	}
	dropLink := func(pl inbPendingLink, reason error) {
		w.stats.ErrorCount++
		w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("commodity %s: item link to %s: %v", pl.commodityUUID, pl.targetUUID, reason))
	}
	errUnresolved := errors.New("the linked item is neither in the backup nor in the group")
	for _, pl := range w.pendingLinks {
		commodityID := w.commodityDBID(pl.commodityUUID)
		if commodityID == "" {
			dropLink(pl, errUnresolved)
			continue
		}
		commodity, err := comReg.Get(w.ctx, commodityID)
		if err != nil {
			dropLink(pl, err)
			continue
		}
		targetID := w.commodityDBID(pl.targetUUID)
		if targetID == "" {
			if !w.holdsTrashedLink(comReg, commodity, pl) {
				dropLink(pl, errUnresolved)
			}
			continue
		}
		commodity.CustomFields = models.CustomFieldValues{pl.fieldID: json.RawMessage(`"` + targetID + `"`)}
		if _, err := comReg.Update(w.ctx, *commodity); err != nil {
			if !errors.Is(err, registry.ErrInvalidCustomFieldValue) && !errors.Is(err, registry.ErrNotFound) {
				return errxtrace.Wrap("failed to set item link", err, errx.Attrs("commodity_id", pl.commodityUUID))
			}
			dropLink(pl, err)
		}
	}
	return nil
}

// holdsTrashedLink reports whether commodity already holds the link pl names,
// to an item in the group's Trash: the restore cannot name that item, but
// the stored link stays.
func (w *inbWalker) holdsTrashedLink(comReg registry.CommodityRegistry, commodity *models.Commodity, pl inbPendingLink) bool {
	var stored string
	if json.Unmarshal(commodity.CustomFields[pl.fieldID], &stored) != nil || stored == "" {
		return false
	}
	target, err := comReg.GetTrashed(w.ctx, stored)
	return err == nil && target.UUID == pl.targetUUID
}

// commodityDBID resolves an archived item UUID to the destination item: one
// this restore created, or one the group already had.
func (w *inbWalker) commodityDBID(uuid string) string {
	if id := w.idMapping.Commodities[uuid]; id != "" {
		return id
	}
	if existing := w.existing.Commodities[uuid]; existing != nil {
		return existing.ID
	}
	return ""
}

// typeID returns the ID of the destination item type of an archived slug,
// or "" when the resolver writes nothing (a dry run) or the group was too
// full to create the type: a template entry for that type would otherwise
// land on the fallback type.
func (r *itemTypeResolver) typeID(ctx context.Context, slug models.CommodityType, stats *types.RestoreStats) (string, error) {
	if r.reg == nil || slug == "" {
		return "", nil
	}
	dest, err := r.resolve(ctx, slug, stats)
	if err != nil {
		return "", err
	}
	if r.fellBack[slug] {
		return "", nil
	}
	t, err := r.reg.GetBySlug(ctx, dest)
	if err != nil {
		return "", errxtrace.Wrap("failed to look up item type", err, errx.Attrs("item_type", string(slug)))
	}
	return t.ID, nil
}
