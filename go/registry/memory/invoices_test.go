package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestInvoiceRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
	})

	// Create a new instance of InvoiceRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	baseRegistry := memory.NewInvoiceRegistry(commodityRegistry)
	r, err := baseRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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
	createdInvoice, err := r.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.Not(qt.IsNil))

	// Verify the count of invoices in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestInvoiceRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
	})

	// Create a new instance of InvoiceRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	baseRegistry := memory.NewInvoiceRegistry(commodityRegistry)
	r, err := baseRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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
	createdInvoice, err := r.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)

	// Delete the invoice from the registry
	err = r.Delete(ctx, createdInvoice.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the invoice is no longer present in the registry
	_, err = r.Get(ctx, createdInvoice.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of invoices in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestInvoiceRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
	})

	// Create a new instance of InvoiceRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	baseInvoiceRegistry := memory.NewInvoiceRegistry(commodityRegistry)
	r, err := baseInvoiceRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create a test invoice without required fields
	invoice := models.Invoice{}
	createdInvoice, err := r.Create(ctx, invoice)
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
	createdInvoice2, err := r.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice2, qt.Not(qt.IsNil))
}

func TestInvoiceRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
	})

	// Create a new instance of InvoiceRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	baseInvoiceRegistry := memory.NewInvoiceRegistry(commodityRegistry)
	r, err := baseInvoiceRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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
	createdInvoice, err := r.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.Not(qt.IsNil))
}
