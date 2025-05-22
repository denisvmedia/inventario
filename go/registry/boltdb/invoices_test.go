package boltdb_test

import (
	"os"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

func setupTestInvoiceRegistry(t *testing.T) (*boltdb.InvoiceRegistry, *boltdb.CommodityRegistry, *boltdb.AreaRegistry, *boltdb.LocationRegistry, func()) {
	c := qt.New(t)

	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "boltdb-test-*")
	c.Assert(err, qt.IsNil)

	// Create a new database in the temporary directory
	db, err := dbx.NewDB(tempDir, "test.db").Open()
	c.Assert(err, qt.IsNil)

	// Create a location registry
	locationRegistry := boltdb.NewLocationRegistry(db)

	// Create an area registry
	areaRegistry := boltdb.NewAreaRegistry(db, locationRegistry)

	// Create a commodity registry
	commodityRegistry := boltdb.NewCommodityRegistry(db, areaRegistry)

	// Create an invoice registry
	invoiceRegistry := boltdb.NewInvoiceRegistry(db, commodityRegistry)

	// Return the registries and a cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return invoiceRegistry, commodityRegistry, areaRegistry, locationRegistry, cleanup
}

func getInvoiceTestSetup(t *testing.T) (registry.InvoiceRegistry, *models.Commodity, func()) {
	c := qt.New(t)
	c.Helper()

	invoiceRegistry, commodityRegistry, areaRegistry, locationRegistry, cleanup := setupTestInvoiceRegistry(t)

	location1, err := locationRegistry.Create(models.Location{
		Name: "Location 1",
	})
	c.Assert(err, qt.IsNil)

	area1, err := areaRegistry.Create(models.Area{
		Name:       "Area 1",
		LocationID: location1.ID,
	})
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		AreaID:    area1.ID,
		Name:      "commodity1",
		ShortName: "commodity1",
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeWhiteGoods,
		Count:     1,
	}

	createdCommodity, err := commodityRegistry.Create(commodity)
	c.Assert(err, qt.IsNil)
	c.Assert(createdCommodity, qt.Not(qt.IsNil))

	return invoiceRegistry, createdCommodity, cleanup
}

func TestInvoiceRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of InvoiceRegistry
	r, createdCommodity, cleanup := getInvoiceTestSetup(t)
	defer cleanup()

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
	createdInvoice, err := r.Create(invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(createdInvoice, qt.Not(qt.IsNil))

	// Verify the count of invoices in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestInvoiceRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of InvoiceRegistry
	r, createdCommodity, cleanup := getInvoiceTestSetup(t)
	defer cleanup()

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
	createdInvoice, err := r.Create(invoice)
	c.Assert(err, qt.IsNil)

	// Delete the invoice from the registry
	err = r.Delete(createdInvoice.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the invoice is no longer present in the registry
	_, err = r.Get(createdInvoice.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of invoices in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestInvoiceRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of InvoiceRegistry
	r, _, cleanup := getInvoiceTestSetup(t)
	defer cleanup()

	// Create a test invoice without required fields
	invoice := models.Invoice{}
	_, err := r.Create(invoice)
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestInvoiceRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of InvoiceRegistry
	r, _, cleanup := getInvoiceTestSetup(t)
	defer cleanup()

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
	_, err := r.Create(invoice)
	c.Assert(err, qt.Not(qt.IsNil))
}
