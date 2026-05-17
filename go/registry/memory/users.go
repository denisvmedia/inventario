package memory

import (
	"context"
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
}

func NewUserRegistry() *UserRegistry {
	return &UserRegistry{
		baseUserRegistry: NewRegistry[models.User, *models.User](),
	}
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
