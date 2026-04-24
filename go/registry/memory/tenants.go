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

// Create wraps the base Create to default the registration mode to closed,
// mirroring the DB-level default on the tenants table.
func (r *TenantRegistry) Create(ctx context.Context, tenant models.Tenant) (*models.Tenant, error) {
	if tenant.RegistrationMode == "" {
		tenant.RegistrationMode = models.RegistrationModeClosed
	}
	return r.baseTenantRegistry.Create(ctx, tenant)
}

// Update wraps the base Update to keep the registration mode consistent with
// the schema: an empty zero-value is normalised to closed before persisting.
func (r *TenantRegistry) Update(ctx context.Context, tenant models.Tenant) (*models.Tenant, error) {
	if tenant.RegistrationMode == "" {
		tenant.RegistrationMode = models.RegistrationModeClosed
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
