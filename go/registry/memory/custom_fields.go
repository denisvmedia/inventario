package memory

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CustomFieldRegistryFactory creates CustomFieldRegistry instances (#2803).
// The values live on the items of the commodity factory, as
// models.Commodity.CustomFields; a field delete or an option removal edits
// them there, standing in for the cascades of the postgres registry. The
// template entries live on the stored definitions, and an entry whose item
// type is gone is left out when it is read, standing in for the cascade of
// an item type delete.
type CustomFieldRegistryFactory struct {
	base        *Registry[models.CustomFieldDefinition, *models.CustomFieldDefinition]
	commodities *CommodityRegistryFactory
	itemTypes   *ItemTypeRegistryFactory
	// mu serializes the writes of fields (Lock) against each other and
	// against the item writes that check values (RLock), as the postgres
	// row locks do. Lock order: the item type lock, then mu, then the
	// commodity map's lock.
	mu *sync.RWMutex
}

// CustomFieldRegistry is the context-aware in-memory registry of custom
// fields.
type CustomFieldRegistry struct {
	*Registry[models.CustomFieldDefinition, *models.CustomFieldDefinition]

	factory *CustomFieldRegistryFactory
}

var (
	_ registry.CustomFieldRegistry        = (*CustomFieldRegistry)(nil)
	_ registry.CustomFieldRegistryFactory = (*CustomFieldRegistryFactory)(nil)
)

// NewCustomFieldRegistryFactory creates the factory and attaches it to
// commodities, whose writes then check their custom field values. itemTypes
// are the types a template entry may name; nil accepts none.
func NewCustomFieldRegistryFactory(commodities *CommodityRegistryFactory, itemTypes *ItemTypeRegistryFactory) *CustomFieldRegistryFactory {
	f := &CustomFieldRegistryFactory{
		base:        NewRegistry[models.CustomFieldDefinition, *models.CustomFieldDefinition](),
		commodities: commodities,
		itemTypes:   itemTypes,
		mu:          &sync.RWMutex{},
	}
	if commodities != nil {
		commodities.customFields = f
	}
	return f
}

func (f *CustomFieldRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CustomFieldRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CustomFieldRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CustomFieldRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return f.newRegistry(user.ID, appctx.GroupIDFromContext(ctx)), nil
}

func (f *CustomFieldRegistryFactory) CreateServiceRegistry() registry.CustomFieldRegistry {
	return f.newRegistry("", "")
}

func (f *CustomFieldRegistryFactory) newRegistry(userID, groupID string) *CustomFieldRegistry {
	return &CustomFieldRegistry{
		Registry: &Registry[models.CustomFieldDefinition, *models.CustomFieldDefinition]{
			items:   f.base.items,
			lock:    f.base.lock,
			userID:  userID,
			groupID: groupID,
		},
		factory: f,
	}
}

// groupFields returns copies of the fields of groupID.
func (f *CustomFieldRegistryFactory) groupFields(groupID string) []*models.CustomFieldDefinition {
	f.base.lock.RLock()
	defer f.base.lock.RUnlock()
	var out []*models.CustomFieldDefinition
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.GroupID == groupID {
			out = append(out, f.copyField(pair.Value))
		}
	}
	return out
}

// copyField returns a copy of d that shares no slice with it, without the
// template entries whose item type is gone.
func (f *CustomFieldRegistryFactory) copyField(d *models.CustomFieldDefinition) *models.CustomFieldDefinition {
	out := *d
	out.Options = slices.Clone(d.Options)
	out.Templates = make([]models.CustomFieldTemplate, 0, len(d.Templates))
	for _, t := range d.Templates {
		if f.itemTypeExists(d.GroupID, t.ItemTypeID) {
			out.Templates = append(out.Templates, models.CustomFieldTemplate{
				ItemTypeID:   t.ItemTypeID,
				DefaultValue: models.JSONValue(slices.Clone([]byte(t.DefaultValue))),
			})
		}
	}
	return &out
}

func (f *CustomFieldRegistryFactory) itemTypeExists(groupID, id string) bool {
	if f.itemTypes == nil {
		return false
	}
	f.itemTypes.base.lock.RLock()
	defer f.itemTypes.base.lock.RUnlock()
	t, ok := f.itemTypes.base.items.Get(id)
	return ok && t.GroupID == groupID
}

// lockForItemWrite holds mu for reading until the returned function is
// called, so no field write runs between checking an item's values and
// storing them.
func (f *CustomFieldRegistryFactory) lockForItemWrite() func() {
	f.mu.RLock()
	return f.mu.RUnlock
}

// fieldsByID returns the fields of groupID among ids, keyed by ID. The
// caller holds mu.
func (f *CustomFieldRegistryFactory) fieldsByID(groupID string, ids []string) map[string]*models.CustomFieldDefinition {
	out := map[string]*models.CustomFieldDefinition{}
	for _, d := range f.groupFields(groupID) {
		if slices.Contains(ids, d.ID) {
			out[d.ID] = d
		}
	}
	return out
}

// rewriteValues replaces the custom field values of every item of groupID,
// in Trash or not, with what fn returns for them; fn returns false to leave
// an item alone. The stored maps are never changed in place: an item gets a
// new map. The caller holds mu for writing.
func (f *CustomFieldRegistryFactory) rewriteValues(groupID string, fn func(models.CustomFieldValues) (models.CustomFieldValues, bool)) {
	if f.commodities == nil {
		return
	}
	base := f.commodities.baseCommodityRegistry
	base.lock.Lock()
	defer base.lock.Unlock()
	for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
		c := pair.Value
		if c == nil || c.GroupID != groupID || len(c.CustomFields) == 0 {
			continue
		}
		if values, changed := fn(c.CustomFields); changed {
			next := *c
			next.CustomFields = values
			base.items.Set(c.ID, &next)
		}
	}
}

// dropFieldValues deletes the values of field id, or with options only the
// values that are one of those options, and returns how many items lost
// one. The caller holds mu for writing.
func (f *CustomFieldRegistryFactory) dropFieldValues(groupID, id string, options []string) int {
	dropped := 0
	f.rewriteValues(groupID, func(values models.CustomFieldValues) (models.CustomFieldValues, bool) {
		raw, ok := values[id]
		if !ok {
			return nil, false
		}
		if options != nil {
			var option string
			if json.Unmarshal(raw, &option) != nil || !slices.Contains(options, option) {
				return nil, false
			}
		}
		next := values.Clone()
		delete(next, id)
		if len(next) == 0 {
			next = nil
		}
		dropped++
		return next, true
	})
	return dropped
}

// dropLinksTo deletes every link to the item itemID, which is gone, as the
// foreign key cascade does in postgres.
func (f *CustomFieldRegistryFactory) dropLinksTo(itemID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	links := map[string][]string{}
	f.base.lock.RLock()
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		if d := pair.Value; d.Type == models.CustomFieldTypeItemLink {
			links[d.GroupID] = append(links[d.GroupID], d.ID)
		}
	}
	f.base.lock.RUnlock()
	target, _ := json.Marshal(itemID)
	for groupID, ids := range links {
		f.rewriteValues(groupID, func(values models.CustomFieldValues) (models.CustomFieldValues, bool) {
			next := values.Clone()
			for _, id := range ids {
				if string(next[id]) == string(target) {
					delete(next, id)
				}
			}
			if len(next) == len(values) {
				return nil, false
			}
			if len(next) == 0 {
				next = nil
			}
			return next, true
		})
	}
}

// groupsCreatedBy returns the groups of tenantID that have a field userID
// created, for the user purge.
func (f *CustomFieldRegistryFactory) groupsCreatedBy(tenantID, userID string) []string {
	f.base.lock.RLock()
	defer f.base.lock.RUnlock()
	seen := map[string]bool{}
	var out []string
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		d := pair.Value
		if d.TenantID == tenantID && d.CreatedByUserID == userID && !seen[d.GroupID] {
			seen[d.GroupID] = true
			out = append(out, d.GroupID)
		}
	}
	return out
}

// reattributeCreator gives the fields userID created in tenantID to the heir
// of their group in heirs and returns how many each group handed over; a
// group without an heir keeps the creator. The template entries live on the
// fields, so they move with them.
func (f *CustomFieldRegistryFactory) reattributeCreator(tenantID, userID string, heirs map[string]string) map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.base.lock.Lock()
	defer f.base.lock.Unlock()
	moved := map[string]int{}
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		d := pair.Value
		heir := heirs[d.GroupID]
		if d.TenantID != tenantID || d.CreatedByUserID != userID || heir == "" {
			continue
		}
		cp := *d
		cp.CreatedByUserID = heir
		pair.Value = &cp
		moved[d.GroupID]++
	}
	return moved
}

// customFieldTemplates is the hand-over view of the item type entries of the
// fields a user created. On postgres each entry is an item_type_custom_fields
// row carrying its field's creator, handed over in an UPDATE of its own; here
// the entries live in the definition, so handing over the field hands them
// over, and this only counts them for the audit row. Call it before the
// fields change hands.
type customFieldTemplates struct {
	fields *CustomFieldRegistryFactory
}

func (t customFieldTemplates) groupsCreatedBy(tenantID, userID string) []string {
	return t.fields.groupsCreatedBy(tenantID, userID)
}

func (t customFieldTemplates) reattributeCreator(tenantID, userID string, heirs map[string]string) map[string]int {
	f := t.fields
	f.mu.RLock()
	defer f.mu.RUnlock()
	f.base.lock.RLock()
	defer f.base.lock.RUnlock()
	counted := map[string]int{}
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		d := pair.Value
		if d.TenantID != tenantID || d.CreatedByUserID != userID || heirs[d.GroupID] == "" {
			continue
		}
		// An entry whose item type is gone has no row on postgres.
		for _, entry := range d.Templates {
			if f.itemTypeExists(d.GroupID, entry.ItemTypeID) {
				counted[d.GroupID]++
			}
		}
	}
	return counted
}

// purgeWhere removes the fields match selects, for the group and tenant
// purgers, which drop the items first.
func (f *CustomFieldRegistryFactory) purgeWhere(match func(*models.CustomFieldDefinition) bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.base.lock.Lock()
	defer f.base.lock.Unlock()
	var ids []string
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		if match(pair.Value) {
			ids = append(ids, pair.Key)
		}
	}
	for _, id := range ids {
		f.base.items.Delete(id)
	}
}

// groupFor is the group a write lands in: the registry's, or for the service
// registry the row's.
func (r *CustomFieldRegistry) groupFor(d *models.CustomFieldDefinition) string {
	if r.groupID != "" {
		return r.groupID
	}
	return d.GroupID
}

func (r *CustomFieldRegistry) Get(ctx context.Context, id string) (*models.CustomFieldDefinition, error) {
	d, err := r.Registry.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get custom field", err)
	}
	return r.factory.copyField(d), nil
}

func (r *CustomFieldRegistry) GetByName(ctx context.Context, name string) (*models.CustomFieldDefinition, error) {
	key := models.CustomFieldNameKey(name)
	defs, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range defs {
		if d.NameKey == key {
			return d, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "CustomField", "name", name))
}

// List returns the fields ordered by name key, then ID.
func (r *CustomFieldRegistry) List(ctx context.Context) ([]*models.CustomFieldDefinition, error) {
	defs, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*models.CustomFieldDefinition, 0, len(defs))
	for _, d := range defs {
		out = append(out, r.factory.copyField(d))
	}
	slices.SortFunc(out, func(a, b *models.CustomFieldDefinition) int {
		if c := strings.Compare(a.NameKey, b.NameKey); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

// nameTaken reports whether another field of groupID has the name key of d.
// The caller holds mu.
func (r *CustomFieldRegistry) nameTaken(groupID string, d *models.CustomFieldDefinition) bool {
	for _, other := range r.factory.groupFields(groupID) {
		if other.NameKey == d.NameKey && other.ID != d.ID {
			return true
		}
	}
	return false
}

// checkTemplates fails with ErrInvalidInput when an entry names no item type
// of groupID.
func (r *CustomFieldRegistry) checkTemplates(groupID string, templates []models.CustomFieldTemplate) error {
	for _, t := range templates {
		if !r.factory.itemTypeExists(groupID, t.ItemTypeID) {
			return errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("field_name", "templates", "item_type_id", t.ItemTypeID))
		}
	}
	return nil
}

func (r *CustomFieldRegistry) Create(ctx context.Context, def models.CustomFieldDefinition) (*models.CustomFieldDefinition, error) {
	def.Normalize()
	if err := def.ValidateFields(ctx); err != nil {
		return nil, errxtrace.Wrap("invalid custom field", err)
	}
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()

	groupID := r.groupFor(&def)
	if len(r.factory.groupFields(groupID)) >= models.MaxCustomFieldsPerGroup {
		return nil, errxtrace.Classify(registry.ErrResourceLimitExceeded, errx.Attrs("limit", models.MaxCustomFieldsPerGroup))
	}
	if r.nameTaken(groupID, &def) {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", def.Name))
	}
	if err := r.checkTemplates(groupID, def.Templates); err != nil {
		return nil, err
	}
	// The store keeps its own templates, so a later change to the caller's
	// slice or default bytes cannot reach it.
	def.Templates = cloneTemplates(def.Templates)
	now := time.Now()
	def.CreatedAt = now
	def.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, def)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create custom field", err)
	}
	return r.factory.copyField(created), nil
}

func (r *CustomFieldRegistry) Update(ctx context.Context, def models.CustomFieldDefinition) (*models.CustomFieldDefinition, error) {
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()

	stored, err := r.Registry.Get(ctx, def.ID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get custom field", err)
	}
	stored = r.factory.copyField(stored)
	def.TenantGroupAwareEntityID = stored.TenantGroupAwareEntityID
	def.Type = stored.Type
	def.SetEntry = stored.SetEntry
	def.CreatedAt = stored.CreatedAt
	def.UpdatedAt = time.Now()
	def.Normalize()
	keepTemplates := def.Templates == nil
	if keepTemplates {
		def.Templates = stored.Templates
	} else {
		def.Templates = cloneTemplates(def.Templates)
	}
	// Defaults naming a dropped option go with it.
	keep := def.OptionIDs()
	var removed []string
	for _, id := range stored.OptionIDs() {
		if !slices.Contains(keep, id) {
			removed = append(removed, id)
		}
	}
	if keepTemplates {
		def.Templates = clearRemovedDefaults(def.Templates, removed)
	}
	if err := def.ValidateFields(ctx); err != nil {
		return nil, errxtrace.Wrap("invalid custom field", err)
	}
	if r.nameTaken(stored.GroupID, &def) {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", def.Name))
	}
	if err := r.checkTemplates(stored.GroupID, def.Templates); err != nil {
		return nil, err
	}
	if len(removed) > 0 {
		r.factory.dropFieldValues(stored.GroupID, def.ID, removed)
	}
	updated, err := r.Registry.UpdateWithUser(ctx, def)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update custom field", err)
	}
	return r.factory.copyField(updated), nil
}

// AddTemplates adds the missing template entries of the field.
func (r *CustomFieldRegistry) AddTemplates(ctx context.Context, id string, itemTypeIDs []string) (int, error) {
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()

	stored, err := r.Registry.Get(ctx, id)
	if err != nil {
		return 0, errxtrace.Wrap("failed to get custom field", err)
	}
	def := *r.factory.copyField(stored)
	var missing []models.CustomFieldTemplate
	for _, typeID := range itemTypeIDs {
		onTemplate := slices.ContainsFunc(def.Templates, func(t models.CustomFieldTemplate) bool { return t.ItemTypeID == typeID })
		listed := slices.ContainsFunc(missing, func(t models.CustomFieldTemplate) bool { return t.ItemTypeID == typeID })
		if !onTemplate && !listed {
			missing = append(missing, models.CustomFieldTemplate{ItemTypeID: typeID})
		}
	}
	if err := r.checkTemplates(def.GroupID, missing); err != nil {
		return 0, err
	}
	if len(missing) == 0 {
		return 0, nil
	}
	def.Templates = append(def.Templates, missing...)
	def.UpdatedAt = time.Now()
	if _, err := r.Registry.UpdateWithUser(ctx, def); err != nil {
		return 0, errxtrace.Wrap("failed to add custom field templates", err)
	}
	return len(missing), nil
}

// cloneTemplates returns a copy of templates that shares no memory with it,
// and an empty list for nil.
func cloneTemplates(templates []models.CustomFieldTemplate) []models.CustomFieldTemplate {
	out := make([]models.CustomFieldTemplate, 0, len(templates))
	for _, t := range templates {
		out = append(out, models.CustomFieldTemplate{
			ItemTypeID:   t.ItemTypeID,
			DefaultValue: models.JSONValue(slices.Clone([]byte(t.DefaultValue))),
		})
	}
	return out
}

func clearRemovedDefaults(templates []models.CustomFieldTemplate, removed []string) []models.CustomFieldTemplate {
	out := make([]models.CustomFieldTemplate, 0, len(templates))
	for _, t := range templates {
		var option string
		if !t.DefaultValue.IsNull() && json.Unmarshal([]byte(t.DefaultValue), &option) == nil && slices.Contains(removed, option) {
			t.DefaultValue = nil
		}
		out = append(out, t)
	}
	return out
}

// Delete deletes a field no item has a value of.
func (r *CustomFieldRegistry) Delete(ctx context.Context, id string) error {
	_, err := r.deleteField(ctx, id, 0)
	return err
}

// DeleteWithValues deletes the field with its values.
func (r *CustomFieldRegistry) DeleteWithValues(ctx context.Context, id string, expected int) (int, error) {
	return r.deleteField(ctx, id, expected)
}

// deleteField deletes the field and returns how many items had a value of
// it; when another number of items than expected has one, it fails with
// ErrCustomFieldInUse instead.
func (r *CustomFieldRegistry) deleteField(ctx context.Context, id string, expected int) (int, error) {
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()

	stored, err := r.Registry.Get(ctx, id)
	if err != nil {
		return 0, errxtrace.Wrap("failed to get custom field", err)
	}
	items := 0
	if r.factory.commodities != nil {
		base := r.factory.commodities.baseCommodityRegistry
		base.lock.RLock()
		for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
			if c := pair.Value; c.GroupID == stored.GroupID && c.CustomFields[id] != nil {
				items++
			}
		}
		base.lock.RUnlock()
	}
	if err := registry.CheckCustomFieldValueCount(id, items, expected); err != nil {
		return 0, err
	}
	r.factory.dropFieldValues(stored.GroupID, id, nil)
	if err := r.Registry.Delete(ctx, id); err != nil {
		return 0, errxtrace.Wrap("failed to delete custom field", err)
	}
	return items, nil
}

func (r *CustomFieldRegistry) Usage(ctx context.Context) (map[string]registry.CustomFieldUsage, error) {
	usage := map[string]registry.CustomFieldUsage{}
	defs, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	if r.factory.commodities == nil {
		return usage, nil
	}
	byID := make(map[string]*models.CustomFieldDefinition, len(defs))
	for _, d := range defs {
		byID[d.ID] = d
	}
	base := r.factory.commodities.baseCommodityRegistry
	base.lock.RLock()
	defer base.lock.RUnlock()
	for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
		c := pair.Value
		for id, raw := range c.CustomFields {
			d := byID[id]
			if d == nil || d.GroupID != c.GroupID {
				continue
			}
			u := usage[id]
			if c.IsTrashed() {
				u.Trashed++
			} else {
				u.Live++
			}
			var option string
			if d.Type == models.CustomFieldTypeSelect && json.Unmarshal(raw, &option) == nil {
				if u.Options == nil {
					u.Options = map[string]int{}
				}
				u.Options[option]++
			}
			usage[id] = u
		}
	}
	return usage, nil
}
