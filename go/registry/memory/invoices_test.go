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

func TestInvoiceRegistry_Create(t *testing.T) {
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

	// Create a commodity first (needed for invoice)
	_, createdCommodity := getCommodityRegistry(c)

	// Create a test invoice
	invoice := models.Invoice{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create a new invoice in the registry
	createdInvoice, err := registrySet.InvoiceRegistry.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.Not(qt.IsNil))

	// Verify the count of invoices in the registry
	count, err := registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestInvoiceRegistry_Delete(t *testing.T) {
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

	// Create a commodity first (needed for invoice)
	_, createdCommodity := getCommodityRegistry(c)

	// Create a test invoice
	invoice := models.Invoice{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create a new invoice in the registry
	createdInvoice, err := registrySet.InvoiceRegistry.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)

	// Delete the invoice from the registry
	err = registrySet.InvoiceRegistry.Delete(ctx, createdInvoice.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the invoice is no longer present in the registry
	_, err = registrySet.InvoiceRegistry.Get(ctx, createdInvoice.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of invoices in the registry
	count, err := registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestInvoiceRegistry_Create_Validation(t *testing.T) {
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

	// Create a test invoice without required fields
	invoice := models.Invoice{}
	createdInvoice, err := registrySet.InvoiceRegistry.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.Not(qt.IsNil))

	invoice = models.Invoice{
		File: &models.File{
			Path:         "test", // Without extension
			OriginalPath: "test.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
		CommodityID: "invalid",
	}
	// Create the invoice - should succeed (no validation in memory registry)
	createdInvoice2, err := registrySet.InvoiceRegistry.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice2, qt.Not(qt.IsNil))
}

func TestInvoiceRegistry_Create_CommodityNotFound(t *testing.T) {
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

	// Create a test invoice with an invalid commodity ID
	invoice := models.Invoice{
		CommodityID: "invalid",
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create the invoice - should succeed (no validation in memory registry)
	createdInvoice, err := registrySet.InvoiceRegistry.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.Not(qt.IsNil))
}
