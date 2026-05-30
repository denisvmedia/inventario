package export

// Generic export-package test helpers shared across both build tags. These are
// intentionally UNTAGGED so the default `.inb` build (e.g. worker_pause_test.go,
// added under #1308) and the legacy_xml_backup tests both compile against them.

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// testUserID is set dynamically when newTestFactorySet creates the test user.
var testUserID string

// testGroupID is the ID of the default LocationGroup created by newTestFactorySet.
var testGroupID string

// newTestFactorySet creates a factory set seeded with a tenant, user, and a
// default LocationGroup (export's FileEntity creation requires a non-empty
// group_id in context — FileEntity is group-scoped).
func newTestFactorySet() *registry.FactorySet {
	factorySet := memory.NewFactorySet()

	// Create user with a server-generated ID and capture it.
	createdUser := must.Must(factorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			// ID is generated server-side for security.
			TenantID: "test-tenant",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		IsActive: true,
	}))
	testUserID = createdUser.ID

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
	testGroupID = createdGroup.ID

	return factorySet
}

// newTestContext creates a context with the test user + group on it.
func newTestContext() context.Context {
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: testUserID},
			TenantID: "test-tenant",
		},
	})
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: testGroupID}, TenantID: "test-tenant"},
	})
}
