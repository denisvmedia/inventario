package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/wk8/go-ordered-map/v2"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type Registry[T any, P registry.PIDable[T]] struct {
	items   *orderedmap.OrderedMap[string, P]
	lock    *sync.RWMutex
	userID  string // For user-aware filtering (non-group models)
	groupID string // For group-aware filtering (group-scoped data models)
}

//go:noinline
func NewRegistry[T any, P registry.PIDable[T]]() *Registry[T, P] {
	return &Registry[T, P]{
		items: orderedmap.New[string, P](),
		lock:  &sync.RWMutex{},
	}
}

func (r *Registry[T, P]) Create(ctx context.Context, item T) (P, error) {
	// If userID is set, use CreateWithUser to ensure user context is applied
	if r.userID != "" {
		return r.CreateWithUser(ctx, item)
	}

	iitem := P(&item)
	// Always generate a new server-side ID for security (ignore any caller-provided ID).
	iitem.SetID(uuid.New().String())
	// Preserve an existing immutable UUID; generate one only when absent.
	if uuidable, ok := any(iitem).(models.UUIDable); ok {
		if uuidable.GetUUID() == "" {
			uuidable.SetUUID(uuid.New().String())
		}
	}

	r.lock.Lock()
	r.items.Set(iitem.GetID(), iitem)
	r.lock.Unlock()

	return iitem, nil
}

func (r *Registry[_, P]) Get(_ context.Context, id string) (P, error) {
	var zero P
	slog.Info("Getting item", "item_id", id, "user_id", r.userID, "item_type", fmt.Sprintf("%T", zero))

	r.lock.RLock()
	item, ok := r.items.Get(id)
	r.lock.RUnlock()
	if !ok {
		return nil, registry.ErrNotFound
	}

	if !r.isItemVisible(item) {
		return nil, registry.ErrNotFound
	}

	vitem := *item
	return &vitem, nil
}

func (r *Registry[_, P]) List(_ context.Context) ([]P, error) {
	items := make([]P, 0, r.items.Len())
	r.lock.RLock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		item := pair.Value

		if !r.isItemVisible(item) {
			continue
		}

		v := *item
		items = append(items, &v)
	}
	r.lock.RUnlock()
	return items, nil
}

func (r *Registry[T, P]) Update(_ context.Context, item T) (P, error) {
	iitem := P(&item)

	r.lock.Lock()
	defer r.lock.Unlock()

	existingItem, ok := r.items.Get(iitem.GetID())
	if !ok {
		return nil, registry.ErrNotFound
	}

	if !r.isItemVisible(existingItem) {
		return nil, registry.ErrNotFound
	}

	// Always overwrite the incoming UUID with the value from the existing record.
	// UUID is immutable after creation; callers must not be able to change it,
	// whether they supply a non-empty value or an empty one.
	if uuidable, ok := any(iitem).(models.UUIDable); ok {
		if existingUUIDable, ok := any(existingItem).(models.UUIDable); ok {
			uuidable.SetUUID(existingUUIDable.GetUUID())
		}
	}

	// Preserve group context from existing entity — GroupID and CreatedByUserID are
	// set by the registry on Create and must not be overwritten by callers.
	if groupAware, ok := any(iitem).(models.GroupAware); ok {
		if existingGroupAware, ok := any(existingItem).(models.GroupAware); ok {
			groupAware.SetGroupID(existingGroupAware.GetGroupID())
		}
	}
	if createdByAware, ok := any(iitem).(models.CreatedByUserAware); ok {
		if existingCreatedBy, ok := any(existingItem).(models.CreatedByUserAware); ok {
			createdByAware.SetCreatedByUserID(existingCreatedBy.GetCreatedByUserID())
		}
	}

	r.items.Set(iitem.GetID(), iitem)
	return &item, nil
}

func (r *Registry[_, P]) Delete(_ context.Context, id string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.userID != "" || r.groupID != "" {
		existingItem, ok := r.items.Get(id)
		if !ok {
			return registry.ErrNotFound
		}

		if !r.isItemVisible(existingItem) {
			return registry.ErrNotFound
		}
	}

	// For non-user-aware registries, just delete (no error if item doesn't exist)
	r.items.Delete(id)
	return nil
}

func (r *Registry[_, P]) Count(_ context.Context) (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if r.userID != "" || r.groupID != "" {
		count := 0
		for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
			if r.isItemVisible(pair.Value) {
				count++
			}
		}
		return count, nil
	}

	return r.items.Len(), nil
}

// isItemVisible checks if an item should be visible to the current registry context.
// For group-scoped entities (GroupAware), it filters by groupID.
// For user-scoped entities (UserAware), it filters by userID.
// For data models accessed without group context, it falls back to createdByUserID.
func (r *Registry[_, P]) isItemVisible(item P) bool {
	// Group-aware filtering takes priority for data models
	if r.groupID != "" {
		if groupAware, ok := any(item).(models.GroupAware); ok {
			return groupAware.GetGroupID() == r.groupID
		}
	}

	// User-aware filtering for non-group models (refresh tokens, settings, etc.)
	if r.userID != "" {
		if userAware, ok := any(item).(models.UserAware); ok {
			return userAware.GetUserID() == r.userID
		}
		// Fallback for group-scoped data models accessed without group context:
		// filter by CreatedByUserID (preserves old user-based isolation behavior during transition)
		if createdByAware, ok := any(item).(models.CreatedByUserAware); ok {
			return createdByAware.GetCreatedByUserID() == r.userID
		}
	}

	// If no filtering context is set, or entity doesn't implement filtering interfaces, it's visible
	return true
}

// User-aware methods that filter by user_id

// SetUserContext is a no-op for Memory as it doesn't use database-level RLS
func (r *Registry[T, P]) SetUserContext(ctx context.Context, userID string) error {
	// Memory doesn't support database-level RLS, so this is a no-op
	// User filtering is done at the application level
	return nil
}

// WithUserContext executes a function with user context (no-op for Memory)
func (r *Registry[T, P]) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	// Memory doesn't support database-level RLS, so just execute the function
	return fn(ctx)
}

// CreateWithUser creates an entity with user context
func (r *Registry[T, P]) CreateWithUser(ctx context.Context, item T) (P, error) {
	// Use registry's userID if available, otherwise extract from context
	userID := r.userID
	if userID == "" {
		userID = registry.UserIDFromContext(ctx)
		if userID == "" {
			return nil, errxtrace.Classify(registry.ErrUserContextRequired)
		}
	}

	iitem := P(&item)

	// Set user_id on the entity if it's UserAware (non-group models)
	if userAware, ok := any(iitem).(models.UserAware); ok {
		userAware.SetUserID(userID)
	}

	// Set created_by_user_id on the entity if it's CreatedByUserAware (group-scoped models)
	if createdByAware, ok := any(iitem).(models.CreatedByUserAware); ok {
		createdByAware.SetCreatedByUserID(userID)
	}

	// Set group_id on the entity if it's GroupAware and we have a groupID
	if r.groupID != "" {
		if groupAware, ok := any(iitem).(models.GroupAware); ok {
			groupAware.SetGroupID(r.groupID)
		}
	}

	// Set tenant_id on the entity if it's TenantAware — get from user in context.
	// The registry itself does not store tenantID, so we derive it from the user context.
	if tenantAware, ok := any(iitem).(models.TenantAware); ok {
		if user := appctx.UserFromContext(ctx); user != nil && user.TenantID != "" {
			tenantAware.SetTenantID(user.TenantID)
		}
	}

	// Always generate a new server-side ID for security (ignore any caller-provided ID).
	iitem.SetID(uuid.New().String())
	// Preserve an existing immutable UUID; generate one only when absent.
	if uuidable, ok := any(iitem).(models.UUIDable); ok {
		if uuidable.GetUUID() == "" {
			uuidable.SetUUID(uuid.New().String())
		}
	}

	r.lock.Lock()
	r.items.Set(iitem.GetID(), iitem)
	r.lock.Unlock()

	return iitem, nil
}

// GetWithUser gets an entity with user context and validates ownership
func (r *Registry[_, P]) GetWithUser(ctx context.Context, id string) (P, error) {
	// Use registry's userID if available, otherwise extract from context
	userID := r.userID
	if userID == "" {
		userID = registry.UserIDFromContext(ctx)
		if userID == "" {
			return nil, errxtrace.Classify(registry.ErrUserContextRequired)
		}
	}

	r.lock.RLock()
	item, ok := r.items.Get(id)
	r.lock.RUnlock()
	if !ok {
		return nil, registry.ErrNotFound
	}

	// Check if the entity belongs to the user
	if userAware, ok := any(item).(models.UserAware); ok {
		if userAware.GetUserID() != userID {
			return nil, registry.ErrNotFound
		}
	}

	vitem := *item
	return &vitem, nil
}

// ListWithUser lists entities with user context filtering
func (r *Registry[_, P]) ListWithUser(ctx context.Context) ([]P, error) {
	// Use registry's userID if available, otherwise extract from context
	userID := r.userID
	if userID == "" {
		userID = registry.UserIDFromContext(ctx)
		if userID == "" {
			return nil, errxtrace.Classify(registry.ErrUserContextRequired)
		}
	}

	var filteredItems []P
	r.lock.RLock()
	// Iterate from the oldest to the newest
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		item := pair.Value

		// Check if the entity belongs to the user
		if userAware, ok := any(item).(models.UserAware); ok {
			if userAware.GetUserID() == userID {
				v := *item
				filteredItems = append(filteredItems, &v)
			}
		} else {
			// If entity is not UserAware, include it (for backward compatibility)
			v := *item
			filteredItems = append(filteredItems, &v)
		}
	}
	r.lock.RUnlock()

	return filteredItems, nil
}

// UpdateWithUser updates an entity with user context
func (r *Registry[T, P]) UpdateWithUser(ctx context.Context, item T) (P, error) {
	// Use registry's userID if available, otherwise extract from context
	userID := r.userID
	if userID == "" {
		userID = registry.UserIDFromContext(ctx)
		if userID == "" {
			return nil, errxtrace.Classify(registry.ErrUserContextRequired)
		}
	}

	iitem := P(&item)

	// Set user_id on the entity if it's UserAware
	if userAware, ok := any(iitem).(models.UserAware); ok {
		userAware.SetUserID(userID)
	}

	// Validate ownership before update
	_, err := r.GetWithUser(ctx, iitem.GetID())
	if err != nil {
		return nil, err
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	existingItem, ok := r.items.Get(iitem.GetID())
	if !ok {
		return nil, registry.ErrNotFound
	}

	// Preserve immutable UUID from existing entity
	if uuidable, ok := any(iitem).(models.UUIDable); ok {
		if existingUUIDable, ok := any(existingItem).(models.UUIDable); ok {
			uuidable.SetUUID(existingUUIDable.GetUUID())
		}
	}

	// Preserve group context from existing entity
	if groupAware, ok := any(iitem).(models.GroupAware); ok {
		if existingGroupAware, ok := any(existingItem).(models.GroupAware); ok {
			groupAware.SetGroupID(existingGroupAware.GetGroupID())
		}
	}
	if createdByAware, ok := any(iitem).(models.CreatedByUserAware); ok {
		if existingCreatedBy, ok := any(existingItem).(models.CreatedByUserAware); ok {
			createdByAware.SetCreatedByUserID(existingCreatedBy.GetCreatedByUserID())
		}
	}

	r.items.Set(iitem.GetID(), iitem)
	return &item, nil
}

// DeleteWithUser deletes an entity with user context
func (r *Registry[_, _]) DeleteWithUser(ctx context.Context, id string) error {
	// Use registry's userID if available, otherwise extract from context
	userID := r.userID
	if userID == "" {
		userID = registry.UserIDFromContext(ctx)
		if userID == "" {
			return errxtrace.Classify(registry.ErrUserContextRequired)
		}
	}

	// Validate ownership before delete
	_, err := r.GetWithUser(ctx, id)
	if err != nil {
		return err
	}

	r.lock.Lock()
	r.items.Delete(id)
	r.lock.Unlock()
	return nil
}

// CountWithUser counts entities with user context filtering
func (r *Registry[_, _]) CountWithUser(ctx context.Context) (int, error) {
	// Use registry's userID if available, otherwise extract from context
	userID := r.userID
	if userID == "" {
		userID = registry.UserIDFromContext(ctx)
		if userID == "" {
			return 0, errxtrace.Classify(registry.ErrUserContextRequired)
		}
	}

	// Get filtered list and return count
	filteredItems, err := r.ListWithUser(ctx)
	if err != nil {
		return 0, err
	}

	return len(filteredItems), nil
}
