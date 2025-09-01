package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestManualRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a commodity first (needed for manual)
	_, createdCommodity := getCommodityRegistry(c)

	// Create a test manual
	manual := models.Manual{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create a new manual in the registry
	createdManual, err := registrySet.ManualRegistry.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.Not(qt.IsNil))

	// Verify the count of manuals in the registry
	count, err := registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestManualRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a commodity first (needed for manual)
	_, createdCommodity := getCommodityRegistry(c)

	// Create a test manual
	manual := models.Manual{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create a new manual in the registry
	createdManual, err := registrySet.ManualRegistry.Create(ctx, manual)
	c.Assert(err, qt.IsNil)

	// Delete the manual from the registry
	err = registrySet.ManualRegistry.Delete(ctx, createdManual.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the manual is no longer present in the registry
	_, err = registrySet.ManualRegistry.Get(ctx, createdManual.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of manuals in the registry
	count, err := registrySet.ManualRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestManualRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a test manual without required fields
	manual := models.Manual{}
	createdManual, err := registrySet.ManualRegistry.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.Not(qt.IsNil))

	manual = models.Manual{
		File: &models.File{
			Path:         "test", // Without extension
			OriginalPath: "test.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
		CommodityID: "invalid",
	}
	// Create the manual - should succeed (no validation in memory registry)
	createdManual2, err := registrySet.ManualRegistry.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual2, qt.Not(qt.IsNil))
}

func TestManualRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
	}

	// Create user in the system first
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create a test manual with an invalid commodity ID
	manual := models.Manual{
		CommodityID: "invalid",
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create the manual - should succeed (no validation in memory registry)
	createdManual, err := registrySet.ManualRegistry.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.Not(qt.IsNil))
}
