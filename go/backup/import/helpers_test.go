package importpkg_test

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// newTestFactorySet builds an in-memory factory set seeded with a tenant, user,
// and a default LocationGroup, returning it alongside the created user's ID and
// group's ID. The group is required because the imported backup's FileEntity is
// group-scoped — createImportFileEntity resolves export.GroupID and the postgres
// registry rejects a create without a group in context. Shared by the import
// test suite across both build tags.
func newTestFactorySet() (factorySet *registry.FactorySet, userID, groupID string) {
	factorySet = memory.NewFactorySet()

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

	createdGroup := must.Must(factorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "test-tenant"},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Test Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           createdUser.ID,
	}))

	return factorySet, createdUser.ID, createdGroup.ID
}
