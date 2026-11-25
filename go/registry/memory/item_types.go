package memory

import (
	"context"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/google/uuid"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// ItemTypeRegistryFactory creates ItemTypeRegistry instances. All registries
// it creates share one backing map and the commodity factory whose items
// name the types: DeleteReassigning moves and counts those items and
// ItemCounts counts them, standing in for the UPDATE and the GROUP BY of
// the postgres registry.
type ItemTypeRegistryFactory struct {
	base        *Registry[models.ItemType, *models.ItemType]
	commodities *CommodityRegistryFactory
	groups      registry.LocationGroupRegistry
	// mu serializes the writes of item types (Lock) against each other and
	// against the item writes that check a type (RLock), as the postgres
	// group lock and row locks do.
	mu *sync.RWMutex
}

// ItemTypeRegistry is the context-aware in-memory registry of item types.
type ItemTypeRegistry struct {
	*Registry[models.ItemType, *models.ItemType]

	factory  *ItemTypeRegistryFactory
	tenantID string
}

var (
	_ registry.ItemTypeRegistry        = (*ItemTypeRegistry)(nil)
	_ registry.ItemTypeRegistryFactory = (*ItemTypeRegistryFactory)(nil)
)

// NewItemTypeRegistryFactory creates the factory and attaches it to
// commodities, whose writes then check the type they name.
func NewItemTypeRegistryFactory(commodities *CommodityRegistryFactory) *ItemTypeRegistryFactory {
	f := &ItemTypeRegistryFactory{
		base:        NewRegistry[models.ItemType, *models.ItemType](),
		commodities: commodities,
		mu:          &sync.RWMutex{},
	}
	if commodities != nil {
		commodities.itemTypes = f
	}
	return f
}

// SetLocationGroupRegistry gives the factory the groups it reads a group's
// creator from, whom the seeded built-in types are attributed to. Without it
// they are attributed to the user whose request seeds them.
func (f *ItemTypeRegistryFactory) SetLocationGroupRegistry(groups registry.LocationGroupRegistry) {
	f.groups = groups
}

func (f *ItemTypeRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ItemTypeRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *ItemTypeRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ItemTypeRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return f.newRegistry(user.ID, user.TenantID, appctx.GroupIDFromContext(ctx)), nil
}

func (f *ItemTypeRegistryFactory) CreateServiceRegistry() registry.ItemTypeRegistry {
	return f.newRegistry("", "", "")
}

func (f *ItemTypeRegistryFactory) newRegistry(userID, tenantID, groupID string) *ItemTypeRegistry {
	return &ItemTypeRegistry{
		Registry: &Registry[models.ItemType, *models.ItemType]{
			items:   f.base.items,
			lock:    f.base.lock,
			userID:  userID,
			groupID: groupID,
		},
		factory:  f,
		tenantID: tenantID,
	}
}

// groupTypes returns copies of the types of groupID. The caller holds f.mu.
func (f *ItemTypeRegistryFactory) groupTypes(groupID string) []*models.ItemType {
	f.base.lock.RLock()
	defer f.base.lock.RUnlock()
	var out []*models.ItemType
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.GroupID == groupID {
			out = append(out, new(*pair.Value))
		}
	}
	return out
}

// findType returns the type of groupID with slug, or nil. The caller holds
// f.mu.
func (f *ItemTypeRegistryFactory) findType(groupID string, slug models.CommodityType) *models.ItemType {
	for _, t := range f.groupTypes(groupID) {
		if t.Slug == slug {
			return t
		}
	}
	return nil
}

// seedLocked inserts the built-in types into groupID when it has none and
// returns how many it inserted. The caller holds f.mu for writing.
func (f *ItemTypeRegistryFactory) seedLocked(ctx context.Context, tenantID, groupID, userID string) int {
	if groupID == "" || len(f.groupTypes(groupID)) > 0 {
		return 0
	}
	creator := userID
	if f.groups != nil {
		if group, err := f.groups.Get(ctx, groupID); err == nil && group.CreatedBy != "" {
			creator = group.CreatedBy
			if tenantID == "" {
				tenantID = group.TenantID
			}
		}
	}
	now := time.Now()
	f.base.lock.Lock()
	defer f.base.lock.Unlock()
	for _, b := range models.BuiltinItemTypes {
		row := models.NewBuiltinItemType(b)
		row.ID = uuid.New().String()
		row.UUID = uuid.New().String()
		row.TenantID = tenantID
		row.GroupID = groupID
		row.CreatedByUserID = creator
		row.CreatedAt = now
		row.UpdatedAt = now
		f.base.items.Set(row.ID, new(row))
	}
	return len(models.BuiltinItemTypes)
}

// lockForItemWrite checks that slug names a type of groupID, seeding the
// built-in types first when the group has none and slug is a built-in one.
// On success it returns with f.mu held for reading, so no type delete runs
// until the item is stored; the caller calls the returned unlock. A write
// without a group or without a type is not checked, as in the postgres
// registry.
func (f *ItemTypeRegistryFactory) lockForItemWrite(ctx context.Context, tenantID, groupID, userID string, slug models.CommodityType) (func(), error) {
	if groupID == "" || slug == "" {
		return func() {}, nil
	}
	f.mu.RLock()
	if f.findType(groupID, slug) != nil {
		return f.mu.RUnlock, nil
	}
	f.mu.RUnlock()
	if !slug.IsBuiltin() {
		return nil, errxtrace.Classify(registry.ErrUnknownItemType, errx.Attrs("type", string(slug)))
	}
	f.mu.Lock()
	f.seedLocked(ctx, tenantID, groupID, userID)
	f.mu.Unlock()
	f.mu.RLock()
	if f.findType(groupID, slug) != nil {
		return f.mu.RUnlock, nil
	}
	f.mu.RUnlock()
	return nil, errxtrace.Classify(registry.ErrUnknownItemType, errx.Attrs("type", string(slug)))
}

// lockForItemUpdate holds the item types read-locked until the returned
// function is called, so a type delete cannot move items while an item
// update reads the item and writes it back. known reports whether slug names
// a type of groupID; a built-in slug seeds a group that has no types first.
func (f *ItemTypeRegistryFactory) lockForItemUpdate(ctx context.Context, tenantID, groupID, userID string, slug models.CommodityType) (unlock func(), known bool) {
	f.mu.RLock()
	if groupID == "" || slug == "" || f.findType(groupID, slug) != nil {
		return f.mu.RUnlock, true
	}
	f.mu.RUnlock()
	if slug.IsBuiltin() {
		f.mu.Lock()
		f.seedLocked(ctx, tenantID, groupID, userID)
		f.mu.Unlock()
	}
	f.mu.RLock()
	return f.mu.RUnlock, f.findType(groupID, slug) != nil
}

// groupFor is the group a write lands in: the registry's, or for the service
// registry the row's.
func (r *ItemTypeRegistry) groupFor(t *models.ItemType) string {
	if r.groupID != "" {
		return r.groupID
	}
	return t.GroupID
}

func (r *ItemTypeRegistry) tenantFor(ctx context.Context, t *models.ItemType) string {
	if r.tenantID != "" {
		return r.tenantID
	}
	if user := appctx.UserFromContext(ctx); user != nil && user.TenantID != "" {
		return user.TenantID
	}
	return t.TenantID
}

// nameTaken reports whether another type of groupID has the name key of t.
// The caller holds f.mu.
func (r *ItemTypeRegistry) nameTaken(groupID string, t *models.ItemType) bool {
	for _, other := range r.factory.groupTypes(groupID) {
		if other.NameKey == t.NameKey && other.ID != t.ID {
			return true
		}
	}
	return false
}

// List returns the types sorted by registry.SortItemTypes, seeding the
// built-in types first into a user registry's group that has none.
func (r *ItemTypeRegistry) List(ctx context.Context) ([]*models.ItemType, error) {
	if r.groupID != "" {
		r.factory.mu.Lock()
		r.factory.seedLocked(ctx, r.tenantID, r.groupID, r.userID)
		r.factory.mu.Unlock()
	}
	types, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}
	registry.SortItemTypes(types)
	return types, nil
}

func (r *ItemTypeRegistry) Create(ctx context.Context, itemType models.ItemType) (*models.ItemType, error) {
	itemType.Normalize()
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()

	groupID := r.groupFor(&itemType)
	r.factory.seedLocked(ctx, r.tenantFor(ctx, &itemType), groupID, r.userID)
	if len(r.factory.groupTypes(groupID)) >= models.MaxItemTypesPerGroup {
		return nil, errxtrace.Classify(registry.ErrItemTypeLimit, errx.Attrs("group_id", groupID, "limit", models.MaxItemTypesPerGroup))
	}
	if r.nameTaken(groupID, &itemType) {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", itemType.Name))
	}
	taken := func(slug models.CommodityType) (bool, error) {
		return r.factory.findType(groupID, slug) != nil, nil
	}
	if itemType.Slug != "" {
		if r.factory.findType(groupID, itemType.Slug) != nil {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("slug", string(itemType.Slug)))
		}
	} else if err := registry.ChooseItemTypeSlug(&itemType, taken); err != nil {
		return nil, err
	}
	now := time.Now()
	itemType.CreatedAt = now
	itemType.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, itemType)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create item type", err)
	}
	// The base registry returns the stored pointer; hand out a copy so a
	// caller's change does not reach the store without Update.
	return new(*created), nil
}

func (r *ItemTypeRegistry) Update(ctx context.Context, itemType models.ItemType) (*models.ItemType, error) {
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()

	existing, err := r.Get(ctx, itemType.ID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get item type", err)
	}
	itemType.Slug = existing.Slug
	itemType.Normalize()
	if r.nameTaken(existing.GroupID, &itemType) {
		return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", itemType.Name))
	}
	itemType.CreatedAt = existing.CreatedAt
	itemType.UpdatedAt = time.Now()
	updated, err := r.Registry.UpdateWithUser(ctx, itemType)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update item type", err)
	}
	return new(*updated), nil
}

func (r *ItemTypeRegistry) Delete(ctx context.Context, id string) error {
	_, err := r.DeleteReassigning(ctx, id, "")
	return err
}

func (r *ItemTypeRegistry) DeleteReassigning(ctx context.Context, id, reassignTo string) (int, error) {
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()

	src, err := r.Get(ctx, id)
	if err != nil {
		return 0, errxtrace.Wrap("failed to get item type", err)
	}
	if len(r.factory.groupTypes(src.GroupID)) <= 1 {
		return 0, errxtrace.Classify(registry.ErrLastItemType, errx.Attrs("item_type_id", id))
	}
	var target *models.ItemType
	if reassignTo != "" {
		t, err := r.Get(ctx, reassignTo)
		if err != nil || t.ID == src.ID || t.GroupID != src.GroupID {
			return 0, errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("reassign_to", reassignTo))
		}
		target = t
	}

	moved := 0
	if r.factory.commodities != nil {
		base := r.factory.commodities.baseCommodityRegistry
		base.lock.Lock()
		used := 0
		for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
			if c := pair.Value; c.GroupID == src.GroupID && c.Type == src.Slug {
				used++
			}
		}
		if used > 0 && target == nil {
			base.lock.Unlock()
			return 0, errxtrace.Classify(registry.ErrItemTypeInUse, errx.Attrs("item_type_id", id, "items", used))
		}
		// Each moved item is a changed copy: Registry.Get copies the stored
		// item after it releases the lock, so the stored one is never
		// written in place.
		for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
			if c := pair.Value; c.GroupID == src.GroupID && c.Type == src.Slug {
				cp := *c
				cp.Type = target.Slug
				pair.Value = &cp
				moved++
			}
		}
		base.lock.Unlock()
	}

	if err := r.Registry.Delete(ctx, id); err != nil {
		return 0, errxtrace.Wrap("failed to delete item type", err)
	}
	return moved, nil
}

func (r *ItemTypeRegistry) GetBySlug(ctx context.Context, slug models.CommodityType) (*models.ItemType, error) {
	types, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "ItemType", "slug", string(slug)))
}

func (r *ItemTypeRegistry) GetByName(ctx context.Context, name string) (*models.ItemType, error) {
	key := models.ItemTypeNameKey(name)
	types, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		if t.NameKey == key {
			return t, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "ItemType", "name", name))
}

func (r *ItemTypeRegistry) ItemCounts(_ context.Context) (map[models.CommodityType]registry.ItemTypeUsage, error) {
	counts := map[models.CommodityType]registry.ItemTypeUsage{}
	if r.factory.commodities == nil {
		return counts, nil
	}
	base := r.factory.commodities.baseCommodityRegistry
	base.lock.RLock()
	defer base.lock.RUnlock()
	for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
		c := pair.Value
		if r.groupID != "" && c.GroupID != r.groupID {
			continue
		}
		usage := counts[c.Type]
		if c.IsTrashed() {
			usage.Trashed++
		} else {
			usage.Live++
		}
		counts[c.Type] = usage
	}
	return counts, nil
}

func (r *ItemTypeRegistry) EnsureBuiltins(ctx context.Context) (int, error) {
	if r.groupID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()
	return r.factory.seedLocked(ctx, r.tenantID, r.groupID, r.userID), nil
}

// BackfillBuiltins seeds every group of the attached group registry that has
// no item types. Without a group registry there is nothing to enumerate.
func (r *ItemTypeRegistry) BackfillBuiltins(ctx context.Context) (int, error) {
	if r.factory.groups == nil {
		return 0, nil
	}
	groups, err := r.factory.groups.List(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to list groups", err)
	}
	r.factory.mu.Lock()
	defer r.factory.mu.Unlock()
	total := 0
	for _, g := range groups {
		total += r.factory.seedLocked(ctx, g.TenantID, g.ID, g.CreatedBy)
	}
	return total, nil
}

// purgeWhere removes the item types match selects without the rules of
// DeleteReassigning, for the group and tenant purgers, which drop the items
// first.
func (f *ItemTypeRegistryFactory) purgeWhere(match func(*models.ItemType) bool) {
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

// groupsCreatedBy returns the groups of tenantID that have an item type
// userID created.
func (f *ItemTypeRegistryFactory) groupsCreatedBy(tenantID, userID string) []string {
	f.base.lock.RLock()
	defer f.base.lock.RUnlock()
	seen := map[string]bool{}
	var out []string
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		t := pair.Value
		if t.TenantID == tenantID && t.CreatedByUserID == userID && !seen[t.GroupID] {
			seen[t.GroupID] = true
			out = append(out, t.GroupID)
		}
	}
	return out
}

// reattributeCreator gives the item types userID created in tenantID to
// heirs[groupID]. A type whose group has no heir keeps its creator. Each
// changed type is a copy, so a reader holding the stored one sees no change.
func (f *ItemTypeRegistryFactory) reattributeCreator(tenantID, userID string, heirs map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.base.lock.Lock()
	defer f.base.lock.Unlock()
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		t := pair.Value
		heir := heirs[t.GroupID]
		if t.TenantID != tenantID || t.CreatedByUserID != userID || heir == "" {
			continue
		}
		cp := *t
		cp.CreatedByUserID = heir
		pair.Value = &cp
	}
}
