package memory

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/wk8/go-ordered-map/v2"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type Registry[T any, P registry.PIDable[T]] struct {
	items *orderedmap.OrderedMap[string, P]
	lock  sync.RWMutex
}

func NewRegistry[T any, P registry.PIDable[T]]() *Registry[T, P] {
	return &Registry[T, P]{
		items: orderedmap.New[string, P](),
	}
}

func (r *Registry[T, P]) Create(_ context.Context, item T) (P, error) {
	iitem := P(&item)
	// Generate a new ID if one is not already provided
	if iitem.GetID() == "" {
		iitem.SetID(uuid.New().String())
	}

	r.lock.Lock()
	r.items.Set(iitem.GetID(), iitem)
	r.lock.Unlock()

	return iitem, nil
}

func (r *Registry[_, P]) Get(_ context.Context, id string) (P, error) {
	r.lock.RLock()
	item, ok := r.items.Get(id)
	r.lock.RUnlock()
	if !ok {
		return nil, registry.ErrNotFound
	}
	vitem := *item
	return &vitem, nil
}

func (r *Registry[_, P]) List(_ context.Context) ([]P, error) {
	items := make([]P, 0, r.items.Len())
	r.lock.RLock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := *pair.Value
		items = append(items, &v)
	}
	r.lock.RUnlock()
	return items, nil
}

func (r *Registry[T, P]) Update(_ context.Context, item T) (P, error) {
	iitem := P(&item)

	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.items.Get(iitem.GetID())
	if !ok {
		return nil, registry.ErrNotFound
	}

	r.items.Set(iitem.GetID(), iitem)
	return &item, nil
}

func (r *Registry[_, _]) Delete(_ context.Context, id string) error {
	r.lock.Lock()
	r.items.Delete(id)
	r.lock.Unlock()
	return nil
}

func (r *Registry[_, _]) Count(_ context.Context) (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.items.Len(), nil
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
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	iitem := P(&item)

	// Set user_id on the entity if it's UserAware
	if userAware, ok := any(iitem).(models.UserAware); ok {
		userAware.SetUserID(userID)
	}

	// Generate a new ID if one is not already provided
	if iitem.GetID() == "" {
		iitem.SetID(uuid.New().String())
	}

	r.lock.Lock()
	r.items.Set(iitem.GetID(), iitem)
	r.lock.Unlock()

	return iitem, nil
}

// GetWithUser gets an entity with user context and validates ownership
func (r *Registry[_, P]) GetWithUser(ctx context.Context, id string) (P, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
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
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	var filteredItems []P
	r.lock.RLock()
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
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
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

	_, ok := r.items.Get(iitem.GetID())
	if !ok {
		return nil, registry.ErrNotFound
	}

	r.items.Set(iitem.GetID(), iitem)
	return &item, nil
}

// DeleteWithUser deletes an entity with user context
func (r *Registry[_, _]) DeleteWithUser(ctx context.Context, id string) error {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
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
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return 0, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Get filtered list and return count
	filteredItems, err := r.ListWithUser(ctx)
	if err != nil {
		return 0, err
	}

	return len(filteredItems), nil
}
