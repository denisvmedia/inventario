package importpkg_test

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// newTestFactorySet builds an in-memory factory set seeded with a tenant + user
// and returns it alongside the created user's ID. Shared by the import test
// suite across both build tags.
func newTestFactorySet() (*registry.FactorySet, string) {
	factorySet := memory.NewFactorySet()

	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		IsActive: true,
	}))

	must.Must(factorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant"},
		Name:     "Test Tenant",
	}))

	return factorySet, createdUser.ID
}
