package memory

import (
	"context"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.LocationGroupRegistry = (*LocationGroupRegistry)(nil)

type baseLocationGroupRegistry = Registry[models.LocationGroup, *models.LocationGroup]

type LocationGroupRegistry struct {
	*baseLocationGroupRegistry
}

func NewLocationGroupRegistry() *LocationGroupRegistry {
	return &LocationGroupRegistry{
		baseLocationGroupRegistry: NewRegistry[models.LocationGroup, *models.LocationGroup](),
	}
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
