package memory

import (
	"context"

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
