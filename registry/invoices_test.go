package registry_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestMemoryInvoiceRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryInvoiceRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := registry.NewMemoryInvoiceRegistry(commodityRegistry)

	// Create a test invoice
	invoice := models.Invoice{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Create a new invoice in the registry
	createdInvoice, err := r.Create(invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.Not(qt.IsNil))

	// Verify the count of invoices in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestMemoryInvoiceRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryInvoiceRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := registry.NewMemoryInvoiceRegistry(commodityRegistry)

	// Create a test invoice
	invoice := models.Invoice{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Create a new invoice in the registry
	createdInvoice, err := r.Create(invoice)
	c.Assert(err, qt.IsNil)

	// Delete the invoice from the registry
	err = r.Delete(createdInvoice.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the invoice is no longer present in the registry
	_, err = r.Get(createdInvoice.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of invoices in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestMemoryInvoiceRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryInvoiceRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := registry.NewMemoryInvoiceRegistry(commodityRegistry)

	// Create a test invoice without required fields
	invoice := models.Invoice{}
	_, err := r.Create(invoice)
	c.Assert(err, qt.Not(qt.IsNil))
	var errs validation.Errors
	c.Assert(err, qt.ErrorAs, &errs)
	c.Assert(errs["File"], qt.Not(qt.IsNil))
	c.Assert(errs["File"].Error(), qt.Equals, "cannot be blank")
	c.Assert(errs["commodity_id"], qt.Not(qt.IsNil))
	c.Assert(errs["commodity_id"].Error(), qt.Equals, "cannot be blank")

	invoice = models.Invoice{
		File: &models.File{
			Path:     "test",
			Ext:      ".png",
			MIMEType: "image/png",
		},
		CommodityID: "invalid",
	}
	// Attempt to create the invoice in the registry and expect a validation error
	_, err = r.Create(invoice)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}

func TestMemoryInvoiceRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryInvoiceRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := registry.NewMemoryInvoiceRegistry(commodityRegistry)

	// Create a test invoice with an invalid commodity ID
	invoice := models.Invoice{
		CommodityID: "invalid",
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Attempt to create the invoice in the registry and expect a commodity not found error
	_, err := r.Create(invoice)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}
