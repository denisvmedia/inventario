package memory

import (
	"context"

	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/internal/errkit"
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
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Email",
		)
	}

	if user.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	if user.TenantID == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "TenantID",
		)
	}

	// Check if a user with the same email already exists
	existingUser, err := r.GetByEmail(ctx, user.TenantID, user.Email)
	if err == nil && existingUser != nil {
		return nil, errkit.WithStack(registry.ErrEmailAlreadyExists,
			"email", user.Email,
		)
	} else if err != nil && err != registry.ErrNotFound {
		return nil, errkit.Wrap(err, "failed to check for existing user")
	}

	// Generate a new server-side ID for security (ignore any user-provided ID)
	generatedID := uuid.New().String()
	user.ID = generatedID

	// Set UserID to self-reference if not already set
	if user.UserID == "" {
		user.UserID = generatedID
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

// ListByRole returns all users with a specific role within a tenant
func (r *UserRegistry) ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error) {
	users, err := r.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var roleUsers []*models.User
	for _, user := range users {
		if user.Role == role {
			roleUsers = append(roleUsers, user)
		}
	}

	return roleUsers, nil
}
