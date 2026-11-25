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
// group's before the first item names one. A field is matched by name, the
// way the group keeps names unique. A match of the same type is reused, its
// templates left as they are, and gets the archived options it lacks, matched
// by label; a match of another type cannot hold the archived values, so the
// field is counted as an error and its values are left out. A field the
// group lacks is created with its archived options and templates. A dry run
// writes nothing and maps nothing.
func (w *inbWalker) restoreCustomFields(archived []types.INBCustomField) error {
	w.customFields = map[string]*restoredCustomField{}
	if len(archived) == 0 || w.options.DryRun || w.proc.factorySet.CustomFieldRegistryFactory == nil {
		return nil
	}
	reg, err := w.proc.factorySet.CustomFieldRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create custom field registry", err)
	}
	for _, a := range archived {
		if a.ID == "" {
			continue
		}
		restored, err := w.restoreCustomField(reg, a)
		if refusal, ok := errors.AsType[customFieldRefusal](err); ok {
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("custom field %s: %v", a.ID, refusal.err))
			continue
		}
		if err != nil {
			return errxtrace.Wrap("failed to restore custom field", err, errx.Attrs("custom_field_id", a.ID))
		}
		w.customFields[a.ID] = restored
	}
	return nil
}

// customFieldRefusal is an archived field the restore reports and moves
// past, as opposed to an error that stops the restore.
type customFieldRefusal struct{ err error }

func (r customFieldRefusal) Error() string { return r.err.Error() }

// restoreCustomField returns the destination of one archived field, or a
// customFieldRefusal.
func (w *inbWalker) restoreCustomField(reg registry.CustomFieldRegistry, a types.INBCustomField) (*restoredCustomField, error) {
	existing, err := reg.GetByName(w.ctx, a.Name)
	switch {
	case err == nil:
		if string(existing.Type) != a.Type {
			return nil, customFieldRefusal{fmt.Errorf("the group has a field named %q of type %s", existing.Name, existing.Type)}
		}
		return w.mergeCustomFieldOptions(reg, existing, a)
	case !errors.Is(err, registry.ErrNotFound):
		return nil, err
	}

	def := models.CustomFieldDefinition{Name: a.Name, Type: models.CustomFieldType(a.Type)}
	for _, o := range a.Options {
		def.Options = append(def.Options, models.CustomFieldOption{ID: o.ID, Label: o.Label})
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
	case errors.Is(err, registry.ErrResourceLimitExceeded), errors.Is(err, registry.ErrInvalidInput):
		return nil, customFieldRefusal{err}
	case err != nil:
		return nil, err
	}
	return newRestoredCustomField(created, a.Options), nil
}

// mergeCustomFieldOptions adds to an existing select field the archived
// options it has no option of the same label for, so the archived values
// that hold them restore.
func (w *inbWalker) mergeCustomFieldOptions(reg registry.CustomFieldRegistry, existing *models.CustomFieldDefinition, a types.INBCustomField) (*restoredCustomField, error) {
	if existing.Type != models.CustomFieldTypeSelect {
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
		if labels[key] {
			continue
		}
		labels[key] = true
		id := o.ID
		if ids[id] {
			id = ""
		}
		merged.Options = append(merged.Options, models.CustomFieldOption{ID: id, Label: o.Label})
		added = true
	}
	if !added {
		return newRestoredCustomField(existing, a.Options), nil
	}
	merged.Normalize()
	if verr := merged.ValidateFields(w.ctx); verr != nil {
		// Too many options: keep the field as it is and restore the values
		// whose option it has.
		w.stats.ErrorCount++
		w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("custom field %s: %v", a.ID, verr))
		return newRestoredCustomField(existing, a.Options), nil
	}
	updated, err := reg.Update(w.ctx, merged)
	if err != nil {
		return nil, err
	}
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
// option of the same label.
func newRestoredCustomField(def *models.CustomFieldDefinition, archived []types.INBCustomFieldOption) *restoredCustomField {
	byLabel := make(map[string]string, len(def.Options))
	for _, o := range def.Options {
		byLabel[models.CustomFieldNameKey(o.Label)] = o.ID
	}
	r := &restoredCustomField{def: def, options: make(map[string]string, len(archived))}
	for _, o := range archived {
		if id := byLabel[models.CustomFieldNameKey(o.Label)]; id != "" {
			r.options[o.ID] = id
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
// is restored. A link to an item neither restored nor already in the group
// is left out, and so is one the destination refuses (an item in Trash).
func (w *inbWalker) applyPendingLinks() error {
	if len(w.pendingLinks) == 0 {
		return nil
	}
	comReg, err := w.proc.factorySet.CommodityRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user commodity registry for item links", err)
	}
	for _, pl := range w.pendingLinks {
		commodityID := w.commodityDBID(pl.commodityUUID)
		targetID := w.commodityDBID(pl.targetUUID)
		if commodityID == "" || targetID == "" {
			continue
		}
		commodity, err := comReg.Get(w.ctx, commodityID)
		if err != nil {
			return errxtrace.Wrap("failed to load commodity for item link", err, errx.Attrs("commodity_id", pl.commodityUUID))
		}
		commodity.CustomFields = models.CustomFieldValues{pl.fieldID: json.RawMessage(`"` + targetID + `"`)}
		if _, err := comReg.Update(w.ctx, *commodity); err != nil {
			if !errors.Is(err, registry.ErrInvalidCustomFieldValue) {
				return errxtrace.Wrap("failed to set item link", err, errx.Attrs("commodity_id", pl.commodityUUID))
			}
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("commodity %s: item link: %v", pl.commodityUUID, err))
		}
	}
	return nil
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
