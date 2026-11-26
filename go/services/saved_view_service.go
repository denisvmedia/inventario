package services

import (
	"context"
	"errors"
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

// SavedViewInput is what a member sets when creating a view. ShowInSidebar
// and ShowOnDashboard are the creator's own placement of it.
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
// as it is. ShowInSidebar and ShowOnDashboard change where the requesting
// member shows the view; any member who sees the view may set them. The
// other fields change the view itself and take CanEdit.
type SavedViewPatch struct {
	Name            *string
	Icon            *models.SavedViewIcon
	Query           *string
	Layout          *models.SavedViewLayout
	Shared          *bool
	ShowInSidebar   *bool
	ShowOnDashboard *bool
}

func (p SavedViewPatch) changesView() bool {
	return p.Name != nil || p.Icon != nil || p.Query != nil || p.Layout != nil || p.Shared != nil
}

func (p SavedViewPatch) changesPlacement() bool {
	return p.ShowInSidebar != nil || p.ShowOnDashboard != nil
}

// SavedViewEntry is a view as one member sees it.
type SavedViewEntry struct {
	View *models.SavedView
	// ShowInSidebar and ShowOnDashboard are where this member shows the
	// view; every member places the views they see on their own.
	ShowInSidebar   bool
	ShowOnDashboard bool
	// Owned is true for the views the member created.
	Owned bool
	// CanEdit and CanDelete say what the member may do with the view.
	CanEdit   bool
	CanDelete bool
	// QueryError explains why the stored query cannot run: a parameter the
	// contract no longer accepts, a query version this server does not
	// know, or a tag filter a rename left contradicting itself (see
	// commodityquery.Query.RenameTag). Such a view is not counted and does not open as a list; it is
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
// delete a view they created. Where a view shows (sidebar, dashboard) is
// each member's own choice for every view they see, so sharing a view puts
// it in nobody else's sidebar.
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

func entryFor(view *models.SavedView, placement models.SavedViewPlacement, actor SavedViewActor) SavedViewEntry {
	return SavedViewEntry{
		View:            view,
		ShowInSidebar:   placement.ShowInSidebar,
		ShowOnDashboard: placement.ShowOnDashboard,
		Owned:           view.CreatedByUserID == actor.UserID,
		CanEdit:         canEdit(view, actor),
		CanDelete:       canDelete(view, actor),
		QueryError:      storedQueryError(view),
	}
}

// canonicalQuery validates a query string for a saved view and returns its
// canonical form. A leading "?" is accepted and dropped. Custom field
// conditions are checked against the group's fields, so a stored query
// always resolves.
func (s *SavedViewService) canonicalQuery(ctx context.Context, raw string) (string, error) {
	q, err := commodityquery.ParseSaved(strings.TrimPrefix(raw, "?"))
	if err != nil {
		return "", err
	}
	if len(q.CustomFields) > 0 {
		defs, err := s.customFields(ctx)
		if err != nil {
			return "", err
		}
		if err := commodityquery.CheckCustomFields(q, defs); err != nil {
			return "", err
		}
	}
	return q.Encode(), nil
}

// patchQuery returns the canonical form of patch.Query, or "" when the patch
// leaves the query as it is.
func (s *SavedViewService) patchQuery(ctx context.Context, patch SavedViewPatch) (string, error) {
	if patch.Query == nil {
		return "", nil
	}
	return s.canonicalQuery(ctx, *patch.Query)
}

// customFields lists the custom fields of the group in ctx.
func (s *SavedViewService) customFields(ctx context.Context) ([]*models.CustomFieldDefinition, error) {
	if s.factorySet.CustomFieldRegistryFactory == nil {
		return nil, nil
	}
	reg, err := s.factorySet.CustomFieldRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create custom field registry", err)
	}
	defs, err := reg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list custom fields", err)
	}
	return defs, nil
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
	placements, err := reg.Placements(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list saved view placements", err)
	}
	out := make([]SavedViewEntry, 0, len(views))
	for _, v := range views {
		out = append(out, entryFor(v, placements[v.ID], actor))
	}
	return out, nil
}

// Get returns one view the member sees.
func (s *SavedViewService) Get(ctx context.Context, actor SavedViewActor, id string) (SavedViewEntry, error) {
	reg, err := s.registry(ctx)
	if err != nil {
		return SavedViewEntry{}, err
	}
	return s.entry(ctx, reg, actor, id)
}

// entry reads one visible view with the member's placement of it.
func (s *SavedViewService) entry(ctx context.Context, reg registry.SavedViewRegistry, actor SavedViewActor, id string) (SavedViewEntry, error) {
	view, err := reg.Get(ctx, id)
	if err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("failed to get saved view", err)
	}
	placements, err := reg.Placements(ctx)
	if err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("failed to list saved view placements", err)
	}
	return entryFor(view, placements[view.ID], actor), nil
}

// checkCreateLimit refuses a view past MaxSavedViewsPerUser.
func checkCreateLimit(t registry.SavedViewTally) error {
	if t.Own >= models.MaxSavedViewsPerUser {
		return errxtrace.Classify(ErrSavedViewLimitReached, errx.Attrs("limit", models.MaxSavedViewsPerUser, "kind", "per_user"))
	}
	return nil
}

// checkShareLimit refuses sharing a view past MaxSharedSavedViews.
func checkShareLimit(t registry.SavedViewTally) error {
	if t.Shared >= models.MaxSharedSavedViews {
		return errxtrace.Classify(ErrSavedViewLimitReached, errx.Attrs("limit", models.MaxSharedSavedViews, "kind", "shared"))
	}
	return nil
}

// Create stores a new view after every view of the group, with the creator's
// placement of it. The query is validated with commodityquery.ParseSaved and
// stored in canonical form. The limits are checked under the registry's write
// lock, so two creates cannot both pass the last free slot.
func (s *SavedViewService) Create(ctx context.Context, actor SavedViewActor, in SavedViewInput) (SavedViewEntry, error) {
	query, err := s.canonicalQuery(ctx, in.Query)
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
		Name:         strings.TrimSpace(in.Name),
		Icon:         in.Icon,
		Entity:       models.SavedViewEntityCommodities,
		Query:        query,
		QueryVersion: models.SavedViewQueryVersion,
		Layout:       in.Layout,
		Shared:       in.Shared,
	}
	if err := view.ValidateWithContext(ctx); err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("invalid saved view", err)
	}

	reg, err := s.registry(ctx)
	if err != nil {
		return SavedViewEntry{}, err
	}
	placement := registry.SavedViewPlacementPatch{
		ShowInSidebar:   new(in.ShowInSidebar),
		ShowOnDashboard: new(in.ShowOnDashboard),
	}
	created, err := reg.CreateChecked(ctx, view, &placement, func(t registry.SavedViewTally) error {
		if err := checkCreateLimit(t); err != nil {
			return err
		}
		if in.Shared {
			return checkShareLimit(t)
		}
		return nil
	})
	if err != nil {
		return SavedViewEntry{}, errxtrace.Wrap("failed to create saved view", err)
	}
	return entryFor(created, models.SavedViewPlacement{
		ShowInSidebar:   in.ShowInSidebar,
		ShowOnDashboard: in.ShowOnDashboard,
	}, actor), nil
}

// Update applies a patch to a view the member sees. The fields of the view
// take CanEdit, checked on the row read under the registry's write lock
// together with the shared limit; the stored query and position are those of
// that row, so a concurrent tag rename or reorder is not written back over.
// A new query is validated and stored in canonical form under the current
// query version, so saving a view that needed an update brings it up to
// date. The placement fields change only the requesting member's placement.
func (s *SavedViewService) Update(ctx context.Context, actor SavedViewActor, id string, patch SavedViewPatch) (SavedViewEntry, error) {
	reg, err := s.registry(ctx)
	if err != nil {
		return SavedViewEntry{}, err
	}
	if patch.changesView() {
		// The placement is written in the view's transaction, so a failed
		// placement write leaves the view as it was.
		var placement *registry.SavedViewPlacementPatch
		if patch.changesPlacement() {
			placement = &registry.SavedViewPlacementPatch{
				ShowInSidebar:   patch.ShowInSidebar,
				ShowOnDashboard: patch.ShowOnDashboard,
			}
		}
		query, err := s.patchQuery(ctx, patch)
		if err != nil {
			return SavedViewEntry{}, err
		}
		updated, err := updateSavedView(ctx, reg, actor, id, patch, query, placement)
		if err != nil {
			return SavedViewEntry{}, err
		}
		// An admin who unshares another member's view no longer sees it:
		// the change is stored, so it answers with the view as written, and
		// there is no placement left to set or view to read again.
		if !updated.Shared && updated.CreatedByUserID != actor.UserID {
			return entryFor(updated, models.SavedViewPlacement{}, actor), nil
		}
		return s.entry(ctx, reg, actor, id)
	}
	if patch.changesPlacement() {
		_, err := reg.SetPlacement(ctx, id, registry.SavedViewPlacementPatch{
			ShowInSidebar:   patch.ShowInSidebar,
			ShowOnDashboard: patch.ShowOnDashboard,
		})
		if err != nil {
			return SavedViewEntry{}, errxtrace.Wrap("failed to place saved view", err)
		}
	}
	return s.entry(ctx, reg, actor, id)
}

// updateSavedView applies the view fields of patch under the registry's
// write lock and returns the view as stored. query is the canonical form of
// patch.Query, when that is set.
func updateSavedView(
	ctx context.Context,
	reg registry.SavedViewRegistry,
	actor SavedViewActor,
	id string,
	patch SavedViewPatch,
	query string,
	placement *registry.SavedViewPlacementPatch,
) (*models.SavedView, error) {
	updated, err := reg.UpdateFunc(ctx, id, placement, func(view *models.SavedView, tally registry.SavedViewTally) error {
		if !canEdit(view, actor) {
			return errxtrace.Classify(ErrSavedViewForbidden, errx.Attrs("reason", "not_editable"))
		}
		if patch.Shared != nil && *patch.Shared && !view.Shared {
			if !canShare(actor) {
				return errxtrace.Classify(ErrSavedViewForbidden, errx.Attrs("reason", "share_requires_user_role"))
			}
			if err := checkShareLimit(tally); err != nil {
				return err
			}
		}
		if patch.Query != nil {
			view.Query = query
			view.QueryVersion = models.SavedViewQueryVersion
		}
		applySavedViewPatch(view, patch)
		if err := view.ValidateWithContext(ctx); err != nil {
			return errxtrace.Wrap("invalid saved view", err)
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to update saved view", err)
	}
	return updated, nil
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

// Counts returns, per view the member sees and shows in their sidebar or on
// their dashboard, how many items its query selects. The counts use the
// filter of the items list (commodityquery.CountEach over the request's
// registry set), so the badge, the dashboard card and the opened list agree;
// they are read from one snapshot without reading rows. A view whose query
// cannot run has no count.
func (s *SavedViewService) Counts(ctx context.Context, set *registry.Set) (map[string]int, error) {
	reg, err := s.registry(ctx)
	if err != nil {
		return nil, err
	}
	views, err := reg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list saved views", err)
	}
	placements, err := reg.Placements(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list saved view placements", err)
	}
	ids := make([]string, 0, len(placements))
	queries := make([]commodityquery.Query, 0, len(placements))
	var defs []*models.CustomFieldDefinition
	defsRead := false
	for _, v := range views {
		p := placements[v.ID]
		if !p.ShowInSidebar && !p.ShowOnDashboard {
			continue
		}
		if v.QueryVersion != models.SavedViewQueryVersion {
			continue
		}
		q, err := commodityquery.ParseSaved(v.Query)
		if err != nil {
			continue
		}
		// One query that cannot resolve would fail the whole batch.
		if len(q.CustomFields) > 0 {
			if !defsRead {
				if defs, err = s.customFields(ctx); err != nil {
					return nil, err
				}
				defsRead = true
			}
			if commodityquery.CheckCustomFields(q, defs) != nil {
				continue
			}
		}
		ids = append(ids, v.ID)
		queries = append(queries, q)
	}
	totals, err := commodityquery.CountEach(ctx, set, queries)
	if err != nil {
		return nil, errxtrace.Wrap("failed to count saved view items", err)
	}
	counts := make(map[string]int, len(ids))
	for i, id := range ids {
		counts[id] = totals[i]
	}
	return counts, nil
}

// rewriteSavedViewTag is the registry.SavedViewTagRewrite of an item tag
// slug rename: a view whose query names from names to instead, so it keeps
// selecting the tag it was saved with. A view whose query cannot run is left
// alone.
func rewriteSavedViewTag(view models.SavedView, from, to string) (string, bool) {
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
}
