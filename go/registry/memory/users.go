package memory

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.UserRegistry = (*UserRegistry)(nil)

type baseUserRegistry = Registry[models.User, *models.User]

type UserRegistry struct {
	*baseUserRegistry
	// refreshTokenRegistry / membershipRegistry, when set, let ListAdminByTenant
	// and CountSessionsByUser compute their cross-table counts. Targeted tests
	// that don't touch the admin surface can leave these nil; the methods
	// degrade to zero counts in that case, matching "no rows" semantics.
	refreshTokenRegistry registry.RefreshTokenRegistry
	membershipRegistry   registry.GroupMembershipRegistry
}

func NewUserRegistry() *UserRegistry {
	return &UserRegistry{
		baseUserRegistry: NewRegistry[models.User, *models.User](),
	}
}

// SetAdminListingRegistries wires the refresh-token + group-membership
// registries used by ListAdminByTenant and CountSessionsByUser.
// NewFactorySet calls this once all three registries exist; tests that
// don't touch the admin surface can skip the wiring.
func (r *UserRegistry) SetAdminListingRegistries(rt registry.RefreshTokenRegistry, gm registry.GroupMembershipRegistry) {
	r.refreshTokenRegistry = rt
	r.membershipRegistry = gm
}

// Create creates a new user with special handling for self-referencing UserID
func (r *UserRegistry) Create(ctx context.Context, user models.User) (*models.User, error) {
	if user.Email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}

	if user.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if user.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	// Check if a user with the same email already exists
	existingUser, err := r.GetByEmail(ctx, user.TenantID, user.Email)
	if err == nil && existingUser != nil {
		return nil, errxtrace.Classify(registry.ErrEmailAlreadyExists, errx.Attrs("email", user.Email))
	} else if err != nil && err != registry.ErrNotFound {
		return nil, errxtrace.Wrap("failed to check for existing user", err)
	}

	// Generate a new server-side ID for security (ignore any user-provided ID)
	generatedID := uuid.New().String()
	user.ID = generatedID
	if user.UUID == "" {
		user.UUID = uuid.New().String()
	}

	// The legacy users.user_id self-FK was removed by issue #1289 Gap B — the
	// row's own id column is authoritative, so nothing else to populate here.

	// Mirror the postgres `default_expr="CURRENT_TIMESTAMP"` on the
	// created_at / updated_at columns — without this, callers that
	// create users via the in-memory backend (dev mode, unit tests,
	// the seed against memory://) get the Go zero time, which renders
	// as "Member since January 1, 1" on /profile. Only stamp when the
	// caller hasn't supplied a value so tests that pin a specific
	// timestamp keep their override.
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = now
	}

	r.lock.Lock()
	r.items.Set(user.ID, &user)
	r.lock.Unlock()

	return &user, nil
}

// GetByEmail returns a user by email within a tenant
func (r *UserRegistry) GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	users, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user.Email == email && user.TenantID == tenantID {
			return user, nil
		}
	}

	return nil, registry.ErrNotFound
}

// ListAdminByTenant mirrors the postgres impl for the in-memory backend:
// filter by tenant, optionally by search-query (email/name) and is_active
// tri-state, sort and paginate, then attach the per-row membership count
// from the linked GroupMembershipRegistry (zero when unwired).
func (r *UserRegistry) ListAdminByTenant(ctx context.Context, tenantID string, opts registry.AdminUserListOptions) ([]*registry.AdminUserListItem, int, error) {
	if tenantID == "" {
		return nil, 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	users, err := r.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	filtered := filterUsers(users, opts.Query, opts.IsActive)
	sortUsers(filtered, opts.SortField, opts.SortDesc)
	total := len(filtered)

	pageRows := paginate(filtered, opts.Page, opts.PerPage)
	if pageRows == nil {
		return nil, total, nil
	}

	items := make([]*registry.AdminUserListItem, 0, len(pageRows))
	for _, u := range pageRows {
		user := *u
		items = append(items, &registry.AdminUserListItem{
			User:                 &user,
			GroupMembershipCount: r.countMembershipsForUser(ctx, u.TenantID, u.ID),
		})
	}
	return items, total, nil
}

// filterUsers applies the case-insensitive ILIKE-style match on
// email/name plus the tri-state is_active filter.
func filterUsers(users []*models.User, query string, isActive *bool) []*models.User {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" && isActive == nil {
		return users
	}
	filtered := users[:0:0]
	for _, u := range users {
		if q != "" {
			haystack := strings.ToLower(u.Email + " " + u.Name)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		if isActive != nil && u.IsActive != *isActive {
			continue
		}
		filtered = append(filtered, u)
	}
	return filtered
}

// sortUsers sorts the slice in place by the requested column. Unknown
// sort fields fall back to email asc with id as the tiebreaker.
//
//revive:disable-next-line:flag-parameter // SortDesc is the natural shape for the public AdminUserListOptions; threading it down keeps the call site readable.
func sortUsers(users []*models.User, field registry.AdminUserSortField, desc bool) {
	if !field.IsValid() {
		field = registry.AdminUserSortEmail
	}
	sort.SliceStable(users, func(i, j int) bool {
		less := userLess(users[i], users[j], field)
		if desc {
			return !less
		}
		return less
	})
}

func userLess(a, b *models.User, field registry.AdminUserSortField) bool {
	switch field {
	case registry.AdminUserSortName:
		if a.Name != b.Name {
			return a.Name < b.Name
		}
	case registry.AdminUserSortCreatedAt:
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
	case registry.AdminUserSortLastLoginAt:
		at := zeroTimeIfNil(a.LastLoginAt)
		bt := zeroTimeIfNil(b.LastLoginAt)
		if !at.Equal(bt) {
			return at.Before(bt)
		}
	case registry.AdminUserSortIsActive:
		if a.IsActive != b.IsActive {
			// false < true under ascending — matches postgres
			// `ORDER BY u.is_active ASC` where false sorts first.
			// Inactive users surface at the top of the ASC listing so
			// operators can triage them; the FE can flip to DESC for
			// active-first.
			return !a.IsActive && b.IsActive
		}
	default:
		if a.Email != b.Email {
			return a.Email < b.Email
		}
	}
	return a.ID < b.ID
}

func zeroTimeIfNil(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func (r *UserRegistry) countMembershipsForUser(ctx context.Context, tenantID, userID string) int {
	if r.membershipRegistry == nil {
		return 0
	}
	memberships, err := r.membershipRegistry.ListByUser(ctx, tenantID, userID)
	if err != nil {
		return 0
	}
	return len(memberships)
}

// CountSessionsByUser counts unrevoked, unexpired refresh tokens for the
// user via the linked refresh-token registry.
//
// Contract: returns (0, nil) when SetAdminListingRegistries has not been
// called — that is, the registry was constructed bare via
// NewUserRegistry() rather than wired through NewFactorySet(). The
// no-op-when-unwired shape exists so legacy tests that don't touch the
// admin surface can stay terse; production callers always go through
// NewFactorySet which wires the linkage, so the silent-zero path is
// unreachable in production. Tests that exercise CountSessionsByUser
// directly must use NewFactorySet().UserRegistry or call
// SetAdminListingRegistries themselves.
func (r *UserRegistry) CountSessionsByUser(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if r.refreshTokenRegistry == nil {
		return 0, nil
	}
	tokens, err := r.refreshTokenRegistry.ListActiveByUserID(ctx, userID)
	if err != nil {
		return 0, errxtrace.Wrap("failed to list active refresh tokens", err)
	}
	return len(tokens), nil
}

// ListByTenant returns all users for a tenant
func (r *UserRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error) {
	users, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var tenantUsers []*models.User
	for _, user := range users {
		if user.TenantID == tenantID {
			tenantUsers = append(tenantUsers, user)
		}
	}

	return tenantUsers, nil
}

// ListSystemAdmins returns every user with is_system_admin = true across
// all tenants. Mirrors the postgres impl which is backed by a partial
// index; memory just filters the full list.
func (r *UserRegistry) ListSystemAdmins(ctx context.Context) ([]*models.User, error) {
	users, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var admins []*models.User
	for _, user := range users {
		if user.IsSystemAdmin {
			admins = append(admins, user)
		}
	}
	return admins, nil
}

// RevokeSystemAdminAtomic clears IsSystemAdmin on the target user while
// holding the registry's write mutex, which serialises the count check
// and the flag flip the same way postgres serialises them under
// pg_advisory_xact_lock. The memory backend has no transactions, so
// holding the registry lock for the duration of read+check+write is the
// equivalent boundary. Idempotent: a non-admin user returns (false, nil).
//
//revive:disable-next-line:flag-parameter
func (r *UserRegistry) RevokeSystemAdminAtomic(_ context.Context, userID string, allowZero bool) (hadFlag bool, err error) {
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	userPtr, ok := r.items.Get(userID)
	if !ok || userPtr == nil {
		return false, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
			"entity_type", "User",
			"entity_id", userID,
		))
	}

	if !userPtr.IsSystemAdmin {
		// Idempotent — already non-admin.
		return false, nil
	}

	if !allowZero {
		// Count admins under the same lock. Iterating the live items
		// map is fine because the lock prevents concurrent mutation.
		count := 0
		for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
			u := pair.Value
			if u != nil && u.IsSystemAdmin {
				count++
			}
		}
		if count <= 1 {
			return true, errxtrace.Classify(registry.ErrLastSystemAdmin, errx.Attrs(
				"user_id", userID,
			))
		}
	}

	// Copy-on-write — the items map stores pointers, so a direct
	// mutation would leak through to callers holding the pointer.
	updated := *userPtr
	updated.IsSystemAdmin = false
	updated.UpdatedAt = time.Now().UTC()
	r.items.Set(userID, &updated)

	return true, nil
}
