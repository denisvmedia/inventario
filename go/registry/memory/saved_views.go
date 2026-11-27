package memory

import (
	"cmp"
	"context"
	"slices"
	"strings"
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

// SavedViewRegistryFactory creates SavedViewRegistry instances with proper
// context.
type SavedViewRegistryFactory struct {
	base *Registry[models.SavedView, *models.SavedView]
	// writeLock serializes the read-modify-write sequences (position and
	// limits on create, update, reorder, rewrite) that the base registry's
	// lock does not cover as one step. It also guards placements.
	writeLock *sync.Mutex
	// placements holds, per view ID and then per user ID, where that member
	// shows the view.
	placements map[string]map[string]models.SavedViewPlacement
}

// SavedViewRegistry is the in-memory registry of saved views. A user
// registry applies the visibility rule of registry.SavedViewRegistry; a
// service registry (userID empty) sees every row.
type SavedViewRegistry struct {
	base       *Registry[models.SavedView, *models.SavedView]
	writeLock  *sync.Mutex
	placements map[string]map[string]models.SavedViewPlacement
	userID     string
	tenantID   string
	groupID    string
}

var (
	_ registry.SavedViewRegistry        = (*SavedViewRegistry)(nil)
	_ registry.SavedViewRegistryFactory = (*SavedViewRegistryFactory)(nil)
)

// NewSavedViewRegistryFactory returns an empty factory.
func NewSavedViewRegistryFactory() *SavedViewRegistryFactory {
	return &SavedViewRegistryFactory{
		base:       NewRegistry[models.SavedView, *models.SavedView](),
		writeLock:  &sync.Mutex{},
		placements: make(map[string]map[string]models.SavedViewPlacement),
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
		writeLock:  f.writeLock,
		placements: f.placements,
		userID:     user.ID,
		tenantID:   user.TenantID,
		groupID:    groupID,
	}, nil
}

func (f *SavedViewRegistryFactory) CreateServiceRegistry() registry.SavedViewRegistry {
	return &SavedViewRegistry{
		base: &Registry[models.SavedView, *models.SavedView]{
			items: f.base.items,
			lock:  f.base.lock,
		},
		writeLock:  f.writeLock,
		placements: f.placements,
	}
}

// purgeUserPlacements deletes every placement userID has in tenantID, for
// the user purger.
func (f *SavedViewRegistryFactory) purgeUserPlacements(tenantID, userID string) {
	f.writeLock.Lock()
	defer f.writeLock.Unlock()
	for viewID, byUser := range f.placements {
		if p, ok := byUser[userID]; ok && p.TenantID == tenantID {
			delete(byUser, userID)
		}
		if len(byUser) == 0 {
			delete(f.placements, viewID)
		}
	}
}

// removeDepartedMember deletes the placements userID has in groupID and the
// personal views userID created there, with their placements, as the
// postgres membership delete does (#2892). Shared views stay.
func (f *SavedViewRegistryFactory) removeDepartedMember(tenantID, groupID, userID string) {
	f.writeLock.Lock()
	defer f.writeLock.Unlock()
	for viewID, byUser := range f.placements {
		if p, ok := byUser[userID]; ok && p.TenantID == tenantID && p.GroupID == groupID {
			delete(byUser, userID)
		}
		if len(byUser) == 0 {
			delete(f.placements, viewID)
		}
	}
	f.base.lock.Lock()
	defer f.base.lock.Unlock()
	var personal []string
	for pair := f.base.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.TenantID == tenantID && v.GroupID == groupID && v.CreatedByUserID == userID && !v.Shared {
			personal = append(personal, pair.Key)
		}
	}
	for _, id := range personal {
		f.base.items.Delete(id)
		delete(f.placements, id)
	}
}

// rewriteTag stores, for every view of groupID, the query rewrite returns
// when it reports a change. TagRegistry.RenameAtomic calls it under the
// rename mutex.
func (f *SavedViewRegistryFactory) rewriteTag(
	ctx context.Context,
	groupID, from, to string,
	rewrite registry.SavedViewTagRewrite,
) error {
	f.writeLock.Lock()
	defer f.writeLock.Unlock()
	group := &Registry[models.SavedView, *models.SavedView]{items: f.base.items, lock: f.base.lock, groupID: groupID}
	views, err := group.List(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, v := range views {
		query, changed := rewrite(*v, from, to)
		if !changed {
			continue
		}
		v.Query = query
		v.UpdatedAt = now
		if _, err := group.Update(ctx, *v); err != nil {
			return errxtrace.Wrap("failed to rewrite saved view query", err, errx.Attrs("saved_view_id", v.ID))
		}
	}
	return nil
}

func errNeedsUser() error {
	return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field", "user"))
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
	return r.create(ctx, view)
}

func (r *SavedViewRegistry) create(ctx context.Context, view models.SavedView) (*models.SavedView, error) {
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

// tally counts the views of the registry's group; writeLock must be held.
func (r *SavedViewRegistry) tally(ctx context.Context) (registry.SavedViewTally, error) {
	all, err := r.groupViews(ctx)
	if err != nil {
		return registry.SavedViewTally{}, err
	}
	var t registry.SavedViewTally
	for _, v := range all {
		if v.CreatedByUserID == r.userID {
			t.Own++
		}
		if v.Shared {
			t.Shared++
		}
	}
	return t, nil
}

func (r *SavedViewRegistry) CreateChecked(
	ctx context.Context,
	view models.SavedView,
	placement *registry.SavedViewPlacementPatch,
	check func(registry.SavedViewTally) error,
) (*models.SavedView, error) {
	if r.userID == "" {
		return nil, errNeedsUser()
	}
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	tally, err := r.tally(ctx)
	if err != nil {
		return nil, err
	}
	if check != nil {
		if err := check(tally); err != nil {
			return nil, err
		}
	}
	created, err := r.create(ctx, view)
	if err != nil {
		return nil, err
	}
	if placement != nil {
		r.storePlacement(created.ID, *placement)
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

// List returns the visible views ordered by position, then name. It holds
// writeLock, so it never sees a reorder half done.
func (r *SavedViewRegistry) List(ctx context.Context) ([]*models.SavedView, error) {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	return r.list(ctx)
}

// list is List for a caller that holds writeLock.
func (r *SavedViewRegistry) list(ctx context.Context) ([]*models.SavedView, error) {
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

// update writes view over the stored one, keeping the fields the registry
// manages; writeLock must be held.
func (r *SavedViewRegistry) update(ctx context.Context, view models.SavedView) (*models.SavedView, error) {
	existing, err := r.Get(ctx, view.ID)
	if err != nil {
		return nil, err
	}
	view.TenantID = existing.TenantID
	view.SortOrder = existing.SortOrder
	view.CreatedAt = existing.CreatedAt
	view.UpdatedAt = time.Now()
	updated, err := r.base.Update(ctx, view)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update saved view", err)
	}
	return updated, nil
}

func (r *SavedViewRegistry) UpdateFunc(
	ctx context.Context,
	id string,
	placement *registry.SavedViewPlacementPatch,
	mutate func(view *models.SavedView, tally registry.SavedViewTally) error,
) (*models.SavedView, error) {
	if r.userID == "" {
		return nil, errNeedsUser()
	}
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	existing, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	tally, err := r.tally(ctx)
	if err != nil {
		return nil, err
	}
	next := *existing
	if err := mutate(&next, tally); err != nil {
		return nil, err
	}
	next.EntityID = existing.EntityID
	next.GroupID = existing.GroupID
	next.CreatedByUserID = existing.CreatedByUserID
	updated, err := r.update(ctx, next)
	if err != nil {
		return nil, err
	}
	if existing.Shared && !updated.Shared {
		for userID := range r.placements[id] {
			if userID != updated.CreatedByUserID {
				delete(r.placements[id], userID)
			}
		}
	}
	if placement != nil && r.visible(updated) {
		r.storePlacement(id, *placement)
	}
	return updated, nil
}

// storePlacement writes the registry user's placement of viewID; writeLock
// must be held.
func (r *SavedViewRegistry) storePlacement(viewID string, patch registry.SavedViewPlacementPatch) models.SavedViewPlacement {
	now := time.Now()
	byUser := r.placements[viewID]
	if byUser == nil {
		byUser = make(map[string]models.SavedViewPlacement)
		r.placements[viewID] = byUser
	}
	p, ok := byUser[r.userID]
	if !ok {
		p = models.SavedViewPlacement{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: uuid.New().String(), UUID: uuid.New().String()},
				TenantID: r.tenantID,
			},
			GroupID:     r.groupID,
			SavedViewID: viewID,
			UserID:      r.userID,
			CreatedAt:   now,
		}
	}
	if patch.ShowInSidebar != nil {
		p.ShowInSidebar = *patch.ShowInSidebar
	}
	if patch.ShowOnDashboard != nil {
		p.ShowOnDashboard = *patch.ShowOnDashboard
	}
	p.UpdatedAt = now
	byUser[r.userID] = p
	return p
}

func (r *SavedViewRegistry) Placements(ctx context.Context) (map[string]models.SavedViewPlacement, error) {
	if r.userID == "" {
		return nil, errNeedsUser()
	}
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	views, err := r.list(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]models.SavedViewPlacement)
	for _, v := range views {
		if p, ok := r.placements[v.ID][r.userID]; ok {
			out[v.ID] = p
		}
	}
	return out, nil
}

func (r *SavedViewRegistry) SetPlacement(
	ctx context.Context,
	viewID string,
	patch registry.SavedViewPlacementPatch,
) (*models.SavedViewPlacement, error) {
	if r.userID == "" {
		return nil, errNeedsUser()
	}
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	if _, err := r.Get(ctx, viewID); err != nil {
		return nil, err
	}
	return new(r.storePlacement(viewID, patch)), nil
}

// Delete removes a visible view and every placement of it.
func (r *SavedViewRegistry) Delete(ctx context.Context, id string) error {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	if err := r.base.Delete(ctx, id); err != nil {
		return err
	}
	delete(r.placements, id)
	return nil
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
	now := time.Now()
	for i, v := range views {
		if v.SortOrder == positions[i] {
			continue
		}
		// The base registry writes the new position as given; update would
		// keep the stored one.
		v.SortOrder = positions[i]
		v.UpdatedAt = now
		if _, err := r.base.Update(ctx, *v); err != nil {
			return errxtrace.Wrap("failed to update saved view position", err)
		}
	}
	return nil
}
