package memory

import (
	"context"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.TenantRegistry = (*TenantRegistry)(nil)

type baseTenantRegistry = Registry[models.Tenant, *models.Tenant]

type TenantRegistry struct {
	*baseTenantRegistry
}

func NewTenantRegistry() *TenantRegistry {
	return &TenantRegistry{
		baseTenantRegistry: NewRegistry[models.Tenant, *models.Tenant](),
	}
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
