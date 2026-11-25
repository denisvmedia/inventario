package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/commodityquery"
)

var (
	// ErrSavedViewForbidden is returned when a member may see a saved view
	// but not make the change asked for: sharing as a viewer, or editing a
	// view shared by someone else without being an admin.
	ErrSavedViewForbidden = errx.NewSentinel("saved view change not allowed")
	// ErrSavedViewLimitReached is returned when a create or a share would go
	// past MaxSavedViewsPerUser or MaxSharedSavedViews.
	ErrSavedViewLimitReached = errx.NewSentinel("saved view limit reached")
)

// SavedViewActor is the member a saved view operation runs for, with their
// role in the group of the request.
type SavedViewActor struct {
	UserID string
	Role   models.GroupRole
}

// SavedViewInput is what a member sets when creating a view.
type SavedViewInput struct {
	Name            string
	Icon            models.SavedViewIcon
	Query           string
	Layout          models.SavedViewLayout
	Shared          bool
	ShowInSidebar   bool
	ShowOnDashboard bool
}

// SavedViewPatch carries the fields a PATCH may change. A nil field is left
// as it is.
type SavedViewPatch struct {
	Name            *string
	Icon            *models.SavedViewIcon
	Query           *string
	Layout          *models.SavedViewLayout
	Shared          *bool
	ShowInSidebar   *bool
	ShowOnDashboard *bool
}

// SavedViewEntry is a view as one member sees it.
type SavedViewEntry struct {
	View *models.SavedView
	// Owned is true for the views the member created.
	Owned bool
	// CanEdit and CanDelete say what the member may do with the view.
	CanEdit   bool
	CanDelete bool
	// QueryError explains why the stored query cannot run: a parameter the
	// contract no longer accepts, or a query version this server does not
	// know. Such a view is not counted and does not open as a list; it is
	// shown as needing an update instead, so it never selects more, or
	// other, items than it was saved with. Nil when the query runs.
	QueryError *commodityquery.Error
}

// SavedViewService is the write and read path of saved views: it validates
// the stored query against the item query contract, applies who may change
// which view, and keeps the limits.
//
// Personal views are private to the member who created them. Any member,
// viewers included, manages their own personal views, since a view grants no
// access to data. Sharing a view with the group takes the user role; the
// member who shared a view keeps editing it while they hold that role, and
// admins and owners may edit or delete any shared view. A member may always
// delete a view they created.
type SavedViewService struct {
	factorySet *registry.FactorySet
}

// NewSavedViewService wires the service to the registry factories.
func NewSavedViewService(fs *registry.FactorySet) *SavedViewService {
	return &SavedViewService{factorySet: fs}
}

func (s *SavedViewService) registry(ctx context.Context) (registry.SavedViewRegistry, error) {
	reg, err := s.factorySet.SavedViewRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create saved view registry", err)
	}
	return reg, nil
}

// canShare reports whether a member may share views with the group.
func canShare(actor SavedViewActor) bool {
	return actor.Role.AtLeast(models.GroupRoleUser)
}

func canEdit(view *models.SavedView, actor SavedViewActor) bool {
	if !view.Shared {
		return view.CreatedByUserID == actor.UserID
	}
	if actor.Role.AtLeast(models.GroupRoleAdmin) {
		return true
	}
	return view.CreatedByUserID == actor.UserID && canShare(actor)
}

func canDelete(view *models.SavedView, actor SavedViewActor) bool {
	if view.CreatedByUserID == actor.UserID {
		return true
	}
	return view.Shared && actor.Role.AtLeast(models.GroupRoleAdmin)
}

// storedQueryError reports why a stored view query cannot run, or nil.
func storedQueryError(view *models.SavedView) *commodityquery.Error {
	if view.QueryVersion != models.SavedViewQueryVersion {
		return &commodityquery.Error{Reason: "was saved in a query version this server does not support"}
	}
	if _, err := commodityquery.ParseSaved(view.Query); err != nil {
		if qErr, ok := errors.AsType[*commodityquery.Error](err); ok {
			return qErr
		}
		return &commodityquery.Error{Reason: err.Error()}
	}
	return nil
}

func entryFor(view *models.SavedView, actor SavedViewActor) SavedViewEntry {
	return SavedViewEntry{
		View:       view,
		Owned:      view.CreatedByUserID == actor.UserID,
		CanEdit:    canEdit(view, actor),
		CanDelete:  canDelete(view, actor),
		QueryError: storedQueryError(view),
	}
}

// canonicalQuery validates a query string for a saved view and returns its
// canonical form. A leading "?" is accepted and dropped.
func canonicalQuery(raw string) (string, error) {
	q, err := commodityquery.ParseSaved(strings.TrimPrefix(raw, "?"))
	if err != nil {
		return "", err
	}
	return q.Encode(), nil
}

// List returns the views the member sees, in order.
func (s *SavedViewService) List(ctx context.Context, actor SavedViewActor) ([]SavedViewEntry, error) {
	reg, err := s.registry(ctx)
	if err != nil {
		return nil, err
	}
	views, err := reg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list saved views", err)
	}
	out := make([]SavedViewEntry, 0, len(views))
	for _, v := range views {
		out = append(out, entryFor(v, actor))
	}
	return out, nil
}

// Get returns one view the member sees.
func (s *SavedViewService) Get(ctx context.Context, actor SavedViewActor, id string) (SavedViewEntry, error) {
	reg, err := s.registry(ctx)
	if err != nil {
		return SavedViewEntry{}, err
	}
	view, err := reg.Get(ctx, id)
	if err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("failed to get saved view", err)
	}
	return entryFor(view, actor), nil
}

// savedViewTally counts, over the views a member sees (all of their own
// and every shared one), the views they created and the shared ones.
type savedViewTally struct {
	own, shared int
}

func tallySavedViews(views []*models.SavedView, actor SavedViewActor) savedViewTally {
	var t savedViewTally
	for _, v := range views {
		if v.CreatedByUserID == actor.UserID {
			t.own++
		}
		if v.Shared {
			t.shared++
		}
	}
	return t
}

func (t savedViewTally) checkCreate() error {
	if t.own >= models.MaxSavedViewsPerUser {
		return errxtrace.Classify(ErrSavedViewLimitReached, errx.Attrs("limit", models.MaxSavedViewsPerUser, "kind", "per_user"))
	}
	return nil
}

func (t savedViewTally) checkShare() error {
	if t.shared >= models.MaxSharedSavedViews {
		return errxtrace.Classify(ErrSavedViewLimitReached, errx.Attrs("limit", models.MaxSharedSavedViews, "kind", "shared"))
	}
	return nil
}

// Create stores a new view after every view of the group. The query is
// validated with commodityquery.ParseSaved and stored in canonical form.
func (s *SavedViewService) Create(ctx context.Context, actor SavedViewActor, in SavedViewInput) (SavedViewEntry, error) {
	query, err := canonicalQuery(in.Query)
	if err != nil {
		return SavedViewEntry{}, err
	}
	if in.Shared && !canShare(actor) {
		return SavedViewEntry{}, errxtrace.Classify(ErrSavedViewForbidden, errx.Attrs("reason", "share_requires_user_role"))
	}
	if in.Icon == "" {
		in.Icon = models.SavedViewIconBookmark
	}
	view := models.SavedView{
		Name:            strings.TrimSpace(in.Name),
		Icon:            in.Icon,
		Entity:          models.SavedViewEntityCommodities,
		Query:           query,
		QueryVersion:    models.SavedViewQueryVersion,
		Layout:          in.Layout,
		Shared:          in.Shared,
		ShowInSidebar:   in.ShowInSidebar,
		ShowOnDashboard: in.ShowOnDashboard,
	}
	if err := view.ValidateWithContext(ctx); err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("invalid saved view", err)
	}

	reg, err := s.registry(ctx)
	if err != nil {
		return SavedViewEntry{}, err
	}
	views, err := reg.List(ctx)
	if err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("failed to list saved views", err)
	}
	tally := tallySavedViews(views, actor)
	if err := tally.checkCreate(); err != nil {
		return SavedViewEntry{}, err
	}
	if in.Shared {
		if err := tally.checkShare(); err != nil {
			return SavedViewEntry{}, err
		}
	}
	created, err := reg.Create(ctx, view)
	if err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("failed to create saved view", err)
	}
	return entryFor(created, actor), nil
}

// Update applies a patch to a view the member may edit. A new query is
// validated and stored in canonical form under the current query version, so
// saving a view that needed an update brings it up to date.
func (s *SavedViewService) Update(ctx context.Context, actor SavedViewActor, id string, patch SavedViewPatch) (SavedViewEntry, error) {
	reg, err := s.registry(ctx)
	if err != nil {
		return SavedViewEntry{}, err
	}
	view, err := reg.Get(ctx, id)
	if err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("failed to get saved view", err)
	}
	if !canEdit(view, actor) {
		return SavedViewEntry{}, errxtrace.Classify(ErrSavedViewForbidden, errx.Attrs("reason", "not_editable"))
	}

	if patch.Query != nil {
		query, err := canonicalQuery(*patch.Query)
		if err != nil {
			return SavedViewEntry{}, err
		}
		view.Query = query
		view.QueryVersion = models.SavedViewQueryVersion
	}
	sharing := patch.Shared != nil && *patch.Shared && !view.Shared
	if sharing {
		if !canShare(actor) {
			return SavedViewEntry{}, errxtrace.Classify(ErrSavedViewForbidden, errx.Attrs("reason", "share_requires_user_role"))
		}
		views, err := reg.List(ctx)
		if err != nil {
			return SavedViewEntry{}, errxtrace.Wrap("failed to list saved views", err)
		}
		if err := tallySavedViews(views, actor).checkShare(); err != nil {
			return SavedViewEntry{}, err
		}
	}
	applySavedViewPatch(view, patch)
	if err := view.ValidateWithContext(ctx); err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("invalid saved view", err)
	}

	updated, err := reg.Update(ctx, *view)
	if err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("failed to update saved view", err)
	}
	return entryFor(updated, actor), nil
}

func applySavedViewPatch(view *models.SavedView, patch SavedViewPatch) {
	if patch.Name != nil {
		view.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Icon != nil {
		view.Icon = *patch.Icon
	}
	if patch.Layout != nil {
		view.Layout = *patch.Layout
	}
	if patch.Shared != nil {
		view.Shared = *patch.Shared
	}
	if patch.ShowInSidebar != nil {
		view.ShowInSidebar = *patch.ShowInSidebar
	}
	if patch.ShowOnDashboard != nil {
		view.ShowOnDashboard = *patch.ShowOnDashboard
	}
}

// Delete removes a view the member may delete.
func (s *SavedViewService) Delete(ctx context.Context, actor SavedViewActor, id string) error {
	reg, err := s.registry(ctx)
	if err != nil {
		return err
	}
	view, err := reg.Get(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get saved view", err)
	}
	if !canDelete(view, actor) {
		return errxtrace.Classify(ErrSavedViewForbidden, errx.Attrs("reason", "not_deletable"))
	}
	if err := reg.Delete(ctx, id); err != nil {
		return errxtrace.Wrap("failed to delete saved view", err)
	}
	return nil
}

// Reorder puts the views named in ids in that order, among the positions they
// hold now; see registry.SavedViewRegistry.Reorder. Every view must be one
// the member may edit.
func (s *SavedViewService) Reorder(ctx context.Context, actor SavedViewActor, ids []string) error {
	reg, err := s.registry(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		view, err := reg.Get(ctx, id)
		if err != nil {
			return errxtrace.Wrap("failed to get saved view", err)
		}
		if !canEdit(view, actor) {
			return errxtrace.Classify(ErrSavedViewForbidden, errx.Attrs("reason", "not_editable"))
		}
	}
	if err := reg.Reorder(ctx, ids); err != nil {
		return errxtrace.Wrap("failed to reorder saved views", err)
	}
	return nil
}

// Counts returns, per view the member sees with ShowInSidebar set, how many
// items its query selects. Each count runs the query the way the items list
// does (commodityquery.List over the request's registry set), so the badge
// and the opened list agree. A view whose query cannot run has no count.
func (s *SavedViewService) Counts(ctx context.Context, set *registry.Set) (map[string]int, error) {
	reg, err := s.registry(ctx)
	if err != nil {
		return nil, err
	}
	views, err := reg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list saved views", err)
	}
	counts := make(map[string]int, len(views))
	for _, v := range views {
		if !v.ShowInSidebar || v.QueryVersion != models.SavedViewQueryVersion {
			continue
		}
		q, err := commodityquery.ParseSaved(v.Query)
		if err != nil {
			continue
		}
		page, err := commodityquery.List(ctx, set, q, 0, 1)
		if err != nil {
			return nil, errxtrace.Wrap("failed to count saved view items", err, errx.Attrs("saved_view_id", v.ID))
		}
		counts[v.ID] = page.Total
	}
	return counts, nil
}

// RenameTag makes every view of the group that names the item tag slug from
// name to instead. It is called after a tag slug rename, so the views keep
// selecting the tag they were saved with. A view whose query cannot run is
// left alone.
func (s *SavedViewService) RenameTag(ctx context.Context, from, to string) (int, error) {
	reg, err := s.registry(ctx)
	if err != nil {
		return 0, err
	}
	n, err := reg.RewriteQueries(ctx, func(view models.SavedView) (string, bool) {
		if view.QueryVersion != models.SavedViewQueryVersion {
			return "", false
		}
		q, err := commodityquery.ParseSaved(view.Query)
		if err != nil {
			return "", false
		}
		renamed, changed := q.RenameTag(from, to)
		if !changed {
			return "", false
		}
		return renamed.Encode(), true
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to rewrite saved view queries", err)
	}
	if n > 0 {
		slog.DebugContext(ctx, "Saved views follow a tag rename", "from", from, "to", to, "views", n)
	}
	return n, nil
}
