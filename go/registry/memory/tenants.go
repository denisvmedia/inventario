package memory

import (
	"context"
	"sort"
	"strings"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.TenantRegistry = (*TenantRegistry)(nil)

type baseTenantRegistry = Registry[models.Tenant, *models.Tenant]

type TenantRegistry struct {
	*baseTenantRegistry
	// userRegistry / groupRegistry, when set, let ListAdmin compute the
	// user_count and group_count columns the admin listing surfaces.
	// Tests that construct the registry directly (without going through
	// NewFactorySet) can leave these nil — counts then degrade to zero,
	// which is the same shape the FE renders when those tables happen to
	// be empty for a tenant.
	userRegistry  registry.UserRegistry
	groupRegistry registry.LocationGroupRegistry
}

func NewTenantRegistry() *TenantRegistry {
	return &TenantRegistry{
		baseTenantRegistry: NewRegistry[models.Tenant, *models.Tenant](),
	}
}

// SetCountRegistries wires the user + location-group registries the
// memory ListAdmin uses to compute its cross-table counts. NewFactorySet
// calls this once all three registries exist; targeted tests that don't
// touch ListAdmin can skip the wiring.
func (r *TenantRegistry) SetCountRegistries(u registry.UserRegistry, g registry.LocationGroupRegistry) {
	r.userRegistry = u
	r.groupRegistry = g
}

// Create wraps the base Create to default the registration mode to closed
// and the plan_id to "unlimited", mirroring the DB-level defaults on the
// tenants table.
func (r *TenantRegistry) Create(ctx context.Context, tenant models.Tenant) (*models.Tenant, error) {
	if tenant.RegistrationMode == "" {
		tenant.RegistrationMode = models.RegistrationModeClosed
	}
	if tenant.PlanID == "" {
		tenant.PlanID = models.PlanUnlimited.ID
	}
	return r.baseTenantRegistry.Create(ctx, tenant)
}

// Update wraps the base Update to keep the registration mode + plan id
// consistent with the schema: empty zero-values are normalised to the
// DB defaults (closed / unlimited) before persisting.
func (r *TenantRegistry) Update(ctx context.Context, tenant models.Tenant) (*models.Tenant, error) {
	if tenant.RegistrationMode == "" {
		tenant.RegistrationMode = models.RegistrationModeClosed
	}
	if tenant.PlanID == "" {
		tenant.PlanID = models.PlanUnlimited.ID
	}
	return r.baseTenantRegistry.Update(ctx, tenant)
}

// GetDefault returns the tenant marked as default (IsDefault == true).
func (r *TenantRegistry) GetDefault(ctx context.Context) (*models.Tenant, error) {
	tenants, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, tenant := range tenants {
		if tenant.IsDefault {
			return tenant, nil
		}
	}

	return nil, registry.ErrNotFound
}

// GetBySlug returns a tenant by its slug
func (r *TenantRegistry) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	tenants, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, tenant := range tenants {
		if tenant.Slug == slug {
			return tenant, nil
		}
	}

	return nil, registry.ErrNotFound
}

// ListAdmin returns the memory equivalent of postgres' admin tenant
// listing — filter, sort and paginate the in-memory rows, then attach
// user_count + group_count from the linked registries (or zero when the
// registries aren't wired). Mirrors the postgres implementation's
// "Total is post-filter, pre-pagination" semantics so callers see one
// invariant across backends.
func (r *TenantRegistry) ListAdmin(ctx context.Context, opts registry.AdminTenantListOptions) ([]*registry.AdminTenantListItem, int, error) {
	tenants, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	filtered := filterTenantsByQuery(tenants, opts.Query)
	sortTenants(filtered, opts.SortField, opts.SortDesc)
	total := len(filtered)

	pageRows := paginate(filtered, opts.Page, opts.PerPage)
	if pageRows == nil {
		return nil, total, nil
	}

	items := make([]*registry.AdminTenantListItem, 0, len(pageRows))
	for _, t := range pageRows {
		userCount, err := r.countUsersForTenant(ctx, t.ID)
		if err != nil {
			return nil, 0, err
		}
		groupCount, err := r.countGroupsForTenant(ctx, t.ID)
		if err != nil {
			return nil, 0, err
		}
		tenant := *t // copy so callers don't share the registry's pointer
		items = append(items, &registry.AdminTenantListItem{
			Tenant:     &tenant,
			UserCount:  userCount,
			GroupCount: groupCount,
		})
	}
	return items, total, nil
}

// filterTenantsByQuery applies the case-insensitive ILIKE-style substring
// match on name/slug/domain. An empty query returns the input unchanged.
func filterTenantsByQuery(tenants []*models.Tenant, query string) []*models.Tenant {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return tenants
	}
	filtered := tenants[:0:0]
	for _, t := range tenants {
		domain := ""
		if t.Domain != nil {
			domain = *t.Domain
		}
		haystack := strings.ToLower(t.Name + " " + t.Slug + " " + domain)
		if strings.Contains(haystack, q) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// sortTenants sorts the slice in place by the requested column.
// Unknown sort fields fall back to name asc. The id tiebreaker is
// ALWAYS ascending regardless of `desc` so pagination is deterministic
// across asc/desc requests and consistent with postgres's
// `ORDER BY t.<col> <dir>, t.id ASC` (id is always ASC there too).
//
// The primary-key direction is implemented by operand-swap on
// tenantPrimaryLess — `!less` would violate strict-weak-ordering for
// equal keys (both directions would return true). Equal primary keys
// fall through to tenantIDLess, which is asc unconditionally.
//
//revive:disable-next-line:flag-parameter // SortDesc is the natural shape for the public AdminTenantListOptions; threading it down via the same field keeps the call site readable.
func sortTenants(tenants []*models.Tenant, field registry.AdminTenantSortField, desc bool) {
	if !field.IsValid() {
		field = registry.AdminTenantSortName
	}
	sort.SliceStable(tenants, func(i, j int) bool {
		a, b := tenants[i], tenants[j]
		// Apply direction only to the primary key.
		var primary int
		switch {
		case tenantPrimaryLess(a, b, field):
			primary = -1
		case tenantPrimaryLess(b, a, field):
			primary = 1
		}
		if desc {
			primary = -primary
		}
		if primary != 0 {
			return primary < 0
		}
		// Equal primary key → id tiebreaker, always ascending.
		return a.ID < b.ID
	})
}

// tenantPrimaryLess is a strict less-than on the chosen field only.
// Returns false when the field values are equal; the id tiebreaker is
// applied by sortTenants directly and stays ascending regardless of
// the sort direction.
func tenantPrimaryLess(a, b *models.Tenant, field registry.AdminTenantSortField) bool {
	switch field {
	case registry.AdminTenantSortSlug:
		return a.Slug < b.Slug
	case registry.AdminTenantSortCreatedAt:
		return a.CreatedAt.Before(b.CreatedAt)
	case registry.AdminTenantSortStatus:
		return string(a.Status) < string(b.Status)
	}
	return a.Name < b.Name
}

// countUsersForTenant returns the user count from the linked registry.
// Unwired (NewTenantRegistry without SetCountRegistries — targeted
// tests that don't touch ListAdmin) returns (0, nil) by design; once
// the linkage is present, registry errors are propagated rather than
// swallowed so a broken backend doesn't masquerade as an empty tenant.
func (r *TenantRegistry) countUsersForTenant(ctx context.Context, tenantID string) (int, error) {
	if r.userRegistry == nil {
		return 0, nil
	}
	users, err := r.userRegistry.ListByTenant(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return len(users), nil
}

// countGroupsForTenant mirrors countUsersForTenant for the location
// group registry.
func (r *TenantRegistry) countGroupsForTenant(ctx context.Context, tenantID string) (int, error) {
	if r.groupRegistry == nil {
		return 0, nil
	}
	groups, err := r.groupRegistry.ListByTenant(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	return len(groups), nil
}

// paginate slices `rows` to the requested page. Returns nil when the
// requested page is past the end of the slice — callers should treat
// this as "no rows for this page" without erroring.
func paginate[T any](rows []T, page, perPage int) []T {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	offset := (page - 1) * perPage
	if offset >= len(rows) {
		return nil
	}
	end := min(offset+perPage, len(rows))
	return rows[offset:end]
}

// GetAdmin mirrors the postgres single-row detail lookup for the
// in-memory backend. Walks the in-memory user / group registries to
// derive the same counts.
func (r *TenantRegistry) GetAdmin(ctx context.Context, tenantID string) (*registry.AdminTenantListItem, error) {
	if tenantID == "" {
		return nil, registry.ErrFieldRequired
	}
	tenant, err := r.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	userCount, err := r.countUsersForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	groupCount, err := r.countGroupsForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	t := *tenant // copy so callers don't share the registry's pointer
	return &registry.AdminTenantListItem{
		Tenant:     &t,
		UserCount:  userCount,
		GroupCount: groupCount,
	}, nil
}

// GetByDomain returns a tenant by its domain
func (r *TenantRegistry) GetByDomain(ctx context.Context, domain string) (*models.Tenant, error) {
	tenants, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, tenant := range tenants {
		if tenant.Domain == nil {
			continue
		}
		if *tenant.Domain == domain {
			return tenant, nil
		}
	}

	return nil, registry.ErrNotFound
}
