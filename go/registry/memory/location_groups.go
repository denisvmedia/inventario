package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.LocationGroupRegistry = (*LocationGroupRegistry)(nil)

type baseLocationGroupRegistry = Registry[models.LocationGroup, *models.LocationGroup]

type LocationGroupRegistry struct {
	*baseLocationGroupRegistry
	// membershipRegistry / tenantRegistry, when set, let ListAdmin /
	// GetAdmin compute member_count and resolve the tenant chip the admin
	// listing surfaces (#1748). Tests that construct the registry directly
	// (without going through NewFactorySet) can leave these nil — counts
	// then degrade to zero and the tenant chip stays nil, mirroring the
	// "empty join result" shape.
	membershipRegistry registry.GroupMembershipRegistry
	tenantRegistry     registry.TenantRegistry
}

func NewLocationGroupRegistry() *LocationGroupRegistry {
	return &LocationGroupRegistry{
		baseLocationGroupRegistry: NewRegistry[models.LocationGroup, *models.LocationGroup](),
	}
}

// SetAdminListingRegistries wires the membership + tenant registries the
// memory ListAdmin / GetAdmin use to compute member_count and resolve the
// tenant chip. NewFactorySet calls this once all three registries exist;
// targeted tests that don't touch the admin surface can skip the wiring.
func (r *LocationGroupRegistry) SetAdminListingRegistries(gm registry.GroupMembershipRegistry, t registry.TenantRegistry) {
	r.membershipRegistry = gm
	r.tenantRegistry = t
}

func (r *LocationGroupRegistry) GetBySlug(_ context.Context, tenantID, slug string) (*models.LocationGroup, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		group := pair.Value
		if group.Slug == slug && group.TenantID == tenantID {
			v := *group
			return &v, nil
		}
	}

	return nil, registry.ErrNotFound
}

func (r *LocationGroupRegistry) ListByTenant(_ context.Context, tenantID string) ([]*models.LocationGroup, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var groups []*models.LocationGroup
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		group := pair.Value
		if group.TenantID == tenantID {
			v := *group
			groups = append(groups, &v)
		}
	}

	return groups, nil
}

// ListAdmin returns the memory equivalent of postgres' admin group
// listing — filter, sort and paginate the in-memory rows, then attach
// member_count from the linked membership registry (or zero when unwired)
// and the owning tenant from the linked tenant registry (or nil when
// unwired) so the cross-tenant admin list carries an owning-tenant chip
// per row without an FE N+1 lookup. Mirrors the postgres "Total is
// post-filter, pre-pagination" semantics so callers see one invariant
// across backends.
func (r *LocationGroupRegistry) ListAdmin(ctx context.Context, opts registry.AdminGroupListOptions) ([]*registry.AdminGroupListItem, int, error) {
	groups, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	filtered := filterGroups(groups, opts.Query, opts.TenantID, opts.Status)
	sortGroups(filtered, opts.SortField, opts.SortDesc)
	total := len(filtered)

	pageRows := paginate(filtered, opts.Page, opts.PerPage)
	if pageRows == nil {
		return nil, total, nil
	}

	items := make([]*registry.AdminGroupListItem, 0, len(pageRows))
	for _, g := range pageRows {
		memberCount, err := r.countMembersForGroup(ctx, g.ID)
		if err != nil {
			return nil, 0, err
		}
		tenant, err := r.resolveTenant(ctx, g.TenantID)
		if err != nil {
			return nil, 0, err
		}
		group := *g // copy so callers don't share the registry's pointer
		items = append(items, &registry.AdminGroupListItem{
			Group:       &group,
			MemberCount: memberCount,
			Tenant:      tenant,
		})
	}
	return items, total, nil
}

// resolveTenant looks up the owning tenant chip from the linked tenant
// registry, returning a copy so callers don't share the registry's
// pointer. Unwired (NewLocationGroupRegistry without
// SetAdminListingRegistries) or an empty tenantID returns (nil, nil) by
// design, mirroring the "empty join result" shape.
//
// A missing tenant row (group orphaned / tenant corrupt) also returns
// (nil, nil) — this mirrors the postgres LEFT JOIN, which keeps the
// orphaned group visible on the cross-tenant admin surface with a nil
// Tenant rather than dropping it. ErrNotFound here is swallowed, not
// propagated, so ListAdmin / GetAdmin still surface the group.
func (r *LocationGroupRegistry) resolveTenant(ctx context.Context, tenantID string) (*models.Tenant, error) {
	if r.tenantRegistry == nil || tenantID == "" {
		return nil, nil
	}
	tenant, err := r.tenantRegistry.Get(ctx, tenantID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	t := *tenant
	return &t, nil
}

// filterGroups applies the case-insensitive ILIKE-style substring match
// on name/slug plus the exact tenant_id and status filters. Empty filters
// are no-ops.
func filterGroups(groups []*models.LocationGroup, query, tenantID, status string) []*models.LocationGroup {
	q := strings.ToLower(strings.TrimSpace(query))
	tenantID = strings.TrimSpace(tenantID)
	status = strings.TrimSpace(status)
	if q == "" && tenantID == "" && status == "" {
		return groups
	}
	filtered := groups[:0:0]
	for _, g := range groups {
		if q != "" {
			haystack := strings.ToLower(g.Name + " " + g.Slug)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		if tenantID != "" && g.TenantID != tenantID {
			continue
		}
		if status != "" && string(g.Status) != status {
			continue
		}
		filtered = append(filtered, g)
	}
	return filtered
}

// sortGroups sorts the slice in place by the requested column. Unknown
// sort fields fall back to name asc. The id tiebreaker is ALWAYS ascending
// regardless of `desc` so pagination is deterministic across asc/desc
// requests and consistent with postgres's
// `ORDER BY g.<col> <dir>, g.id ASC`.
//
//revive:disable-next-line:flag-parameter // SortDesc is the natural shape for the public AdminGroupListOptions; threading it down keeps the call site readable.
func sortGroups(groups []*models.LocationGroup, field registry.AdminGroupSortField, desc bool) {
	if !field.IsValid() {
		field = registry.AdminGroupSortName
	}
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		var primary int
		switch {
		case groupPrimaryLess(a, b, field):
			primary = -1
		case groupPrimaryLess(b, a, field):
			primary = 1
		}
		if desc {
			primary = -primary
		}
		if primary != 0 {
			return primary < 0
		}
		return a.ID < b.ID
	})
}

// groupPrimaryLess is a strict less-than on the chosen field only.
// Returns false when the field values are equal; the id tiebreaker is
// applied by sortGroups directly and stays ascending regardless of the
// sort direction.
func groupPrimaryLess(a, b *models.LocationGroup, field registry.AdminGroupSortField) bool {
	switch field {
	case registry.AdminGroupSortSlug:
		return a.Slug < b.Slug
	case registry.AdminGroupSortCreatedAt:
		return a.CreatedAt.Before(b.CreatedAt)
	case registry.AdminGroupSortStatus:
		return string(a.Status) < string(b.Status)
	}
	return a.Name < b.Name
}

// countMembersForGroup returns the member count from the linked
// membership registry. Unwired (NewLocationGroupRegistry without
// SetAdminListingRegistries) returns (0, nil) by design; once wired,
// registry errors propagate rather than being swallowed.
func (r *LocationGroupRegistry) countMembersForGroup(ctx context.Context, groupID string) (int, error) {
	if r.membershipRegistry == nil {
		return 0, nil
	}
	return r.membershipRegistry.CountByGroup(ctx, groupID)
}

// GetAdmin mirrors the postgres single-row detail lookup for the
// in-memory backend: load the group, attach member_count and resolve the
// owning tenant chip from the linked registries.
func (r *LocationGroupRegistry) GetAdmin(ctx context.Context, groupID string) (*registry.AdminGroupDetail, error) {
	if groupID == "" {
		return nil, registry.ErrFieldRequired
	}
	group, err := r.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return r.buildAdminGroupDetail(ctx, group)
}

// buildAdminGroupDetail shapes an AdminGroupDetail (group copy +
// member_count + tenant chip) from an already-loaded group. Shared by
// GetAdmin and MarkPendingDeletionAdmin so the post-delete row carries
// exactly the shape the detail handler renders. The supplied group is
// copied so callers don't share the registry's pointer.
func (r *LocationGroupRegistry) buildAdminGroupDetail(ctx context.Context, group *models.LocationGroup) (*registry.AdminGroupDetail, error) {
	memberCount, err := r.countMembersForGroup(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	tenant, err := r.resolveTenant(ctx, group.TenantID)
	if err != nil {
		return nil, err
	}
	g := *group // copy so callers don't share the registry's pointer
	return &registry.AdminGroupDetail{
		Group:       &g,
		MemberCount: memberCount,
		Tenant:      tenant,
	}, nil
}

// MarkPendingDeletionAdmin flips a group to pending_deletion for the
// cross-tenant admin soft-delete (#1748). The status-transition logic is
// identical to GroupService.InitiateGroupDeletion so the group_purge_worker
// finishes the hard-delete with no parallel code path. The registry's
// write lock is held for the whole read-decide-write so two concurrent
// admin deletes can't both observe `active`, and the returned detail row
// is built from the post-transition copy under that same lock — no
// follow-up GetAdmin that could race the purge worker. Idempotent: an
// already-pending group returns (detail, true, nil) without re-writing.
func (r *LocationGroupRegistry) MarkPendingDeletionAdmin(ctx context.Context, groupID string) (*registry.AdminGroupDetail, bool, error) {
	if groupID == "" {
		return nil, false, registry.ErrFieldRequired
	}

	post, alreadyPending, err := r.applyPendingDeletion(groupID)
	if err != nil {
		return nil, false, err
	}
	// buildAdminGroupDetail touches the membership / tenant registries
	// (their own locks), never r.lock, so it is safe to call after the
	// write lock above has been released.
	detail, err := r.buildAdminGroupDetail(ctx, post)
	if err != nil {
		return nil, false, err
	}
	return detail, alreadyPending, nil
}

// applyPendingDeletion performs the locked read-decide-write half of the
// admin soft-delete and returns a copy of the post-transition group. The
// write lock is held only for the mutation; detail-shaping happens after
// it is released so we don't hold r.lock across the membership / tenant
// registry lookups.
func (r *LocationGroupRegistry) applyPendingDeletion(groupID string) (*models.LocationGroup, bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	group, ok := r.items.Get(groupID)
	if !ok {
		return nil, false, registry.ErrNotFound
	}
	if group.Status == models.LocationGroupStatusPendingDeletion {
		post := *group
		return &post, true, nil
	}
	updated := *group
	updated.Status = models.LocationGroupStatusPendingDeletion
	updated.UpdatedAt = time.Now()
	r.items.Set(groupID, &updated)
	post := updated
	return &post, false, nil
}
