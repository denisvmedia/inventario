package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestInvoiceRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of InvoiceRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := memory.NewInvoiceRegistry(commodityRegistry)

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
	ctx := context.Background()

	// Create a new instance of InvoiceRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := memory.NewInvoiceRegistry(commodityRegistry)

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
	ctx := context.Background()

	// Create a new instance of InvoiceRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := memory.NewInvoiceRegistry(commodityRegistry)

	// Create a test invoice without required fields
	invoice := models.Invoice{}
	_, err := r.Create(ctx, invoice)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")

	invoice = models.Invoice{
		File: &models.File{
			Path:         "test", // Without extension
			OriginalPath: "test.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
		CommodityID: "invalid",
	}
	// Attempt to create the invoice in the registry and expect a validation error
	_, err = r.Create(ctx, invoice)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}

func TestInvoiceRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of InvoiceRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := memory.NewInvoiceRegistry(commodityRegistry)

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

	// Attempt to create the invoice in the registry and expect a commodity not found error
	_, err := r.Create(ctx, invoice)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}
