package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// SavedViewRegistryFactory creates SavedViewRegistry instances with proper
// context.
type SavedViewRegistryFactory struct {
	base *Registry[models.SavedView, *models.SavedView]
	// writeLock serializes the read-modify-write sequences (position on
	// create, reorder, rewrite) that the base registry's lock does not cover
	// as one step.
	writeLock *sync.Mutex
}

// SavedViewRegistry is the in-memory registry of saved views. A user
// registry applies the visibility rule of registry.SavedViewRegistry; a
// service registry (userID empty) sees every row.
type SavedViewRegistry struct {
	base      *Registry[models.SavedView, *models.SavedView]
	writeLock *sync.Mutex
	userID    string
	groupID   string
}

var (
	_ registry.SavedViewRegistry        = (*SavedViewRegistry)(nil)
	_ registry.SavedViewRegistryFactory = (*SavedViewRegistryFactory)(nil)
)

func NewSavedViewRegistryFactory() *SavedViewRegistryFactory {
	return &SavedViewRegistryFactory{
		base:      NewRegistry[models.SavedView, *models.SavedView](),
		writeLock: &sync.Mutex{},
	}
}

func (f *SavedViewRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.SavedViewRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *SavedViewRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.SavedViewRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	groupID := appctx.GroupIDFromContext(ctx)
	return &SavedViewRegistry{
		base: &Registry[models.SavedView, *models.SavedView]{
			items:   f.base.items,
			lock:    f.base.lock,
			userID:  user.ID,
			groupID: groupID,
		},
		writeLock: f.writeLock,
		userID:    user.ID,
		groupID:   groupID,
	}, nil
}

func (f *SavedViewRegistryFactory) CreateServiceRegistry() registry.SavedViewRegistry {
	return &SavedViewRegistry{
		base: &Registry[models.SavedView, *models.SavedView]{
			items: f.base.items,
			lock:  f.base.lock,
		},
		writeLock: f.writeLock,
	}
}

func (r *SavedViewRegistry) visible(v *models.SavedView) bool {
	return r.userID == "" || v.VisibleTo(r.userID)
}

// groupViews returns every view of the registry's group, visible or not.
func (r *SavedViewRegistry) groupViews(ctx context.Context) ([]*models.SavedView, error) {
	return r.base.List(ctx)
}

func (r *SavedViewRegistry) Create(ctx context.Context, view models.SavedView) (*models.SavedView, error) {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	all, err := r.groupViews(ctx)
	if err != nil {
		return nil, err
	}
	next := 0
	for _, v := range all {
		next = max(next, v.SortOrder+1)
	}
	now := time.Now()
	view.SortOrder = next
	view.CreatedAt = now
	view.UpdatedAt = now
	// The base registry stamps the creator, group and tenant from the
	// context when it has a user.
	created, err := r.base.Create(ctx, view)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create saved view", err)
	}
	return created, nil
}

func (r *SavedViewRegistry) Get(ctx context.Context, id string) (*models.SavedView, error) {
	v, err := r.base.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !r.visible(v) {
		return nil, registry.ErrNotFound
	}
	return v, nil
}

// List returns the visible views ordered by position, then name.
func (r *SavedViewRegistry) List(ctx context.Context) ([]*models.SavedView, error) {
	all, err := r.groupViews(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*models.SavedView, 0, len(all))
	for _, v := range all {
		if r.visible(v) {
			out = append(out, v)
		}
	}
	sortSavedViews(out)
	return out, nil
}

// sortSavedViews orders views as the postgres registry does: position, then
// name without regard to case, then id so equal rows keep a stable order.
func sortSavedViews(views []*models.SavedView) {
	slices.SortStableFunc(views, func(a, b *models.SavedView) int {
		return cmp.Or(
			cmp.Compare(a.SortOrder, b.SortOrder),
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.ID, b.ID),
		)
	})
}

func (r *SavedViewRegistry) Count(ctx context.Context) (int, error) {
	views, err := r.List(ctx)
	if err != nil {
		return 0, err
	}
	return len(views), nil
}

func (r *SavedViewRegistry) Update(ctx context.Context, view models.SavedView) (*models.SavedView, error) {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	return r.update(ctx, view)
}

func (r *SavedViewRegistry) update(ctx context.Context, view models.SavedView) (*models.SavedView, error) {
	existing, err := r.Get(ctx, view.ID)
	if err != nil {
		return nil, err
	}
	view.TenantID = existing.TenantID
	view.CreatedAt = existing.CreatedAt
	view.UpdatedAt = time.Now()
	updated, err := r.base.Update(ctx, view)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update saved view", err)
	}
	return updated, nil
}

func (r *SavedViewRegistry) Delete(ctx context.Context, id string) error {
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	return r.base.Delete(ctx, id)
}

func (r *SavedViewRegistry) Reorder(ctx context.Context, ids []string) error {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	views := make([]*models.SavedView, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	positions := make([]int, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			return registry.ErrNotFound
		}
		seen[id] = struct{}{}
		v, err := r.Get(ctx, id)
		if err != nil {
			return err
		}
		views = append(views, v)
		positions = append(positions, v.SortOrder)
	}
	slices.Sort(positions)
	for i, v := range views {
		if v.SortOrder == positions[i] {
			continue
		}
		v.SortOrder = positions[i]
		if _, err := r.update(ctx, *v); err != nil {
			return err
		}
	}
	return nil
}

func (r *SavedViewRegistry) RewriteQueries(ctx context.Context, rewrite func(view models.SavedView) (string, bool)) (int, error) {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	all, err := r.groupViews(ctx)
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, v := range all {
		query, ok := rewrite(*v)
		if !ok {
			continue
		}
		v.Query = query
		v.UpdatedAt = time.Now()
		if _, err := r.base.Update(ctx, *v); err != nil {
			return changed, errxtrace.Wrap("failed to rewrite saved view query", err)
		}
		changed++
	}
	return changed, nil
}
