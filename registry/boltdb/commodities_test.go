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

func setupTestCommodityRegistry(t *testing.T) (*boltdb.CommodityRegistry, *boltdb.AreaRegistry, *boltdb.LocationRegistry, func()) {
	c := qt.New(t)
	c.Helper()

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

	// Create a settings registry
	settingsRegistry := boltdb.NewSettingsRegistry(db)
	err = settingsRegistry.Patch("system.main_currency", "USD")
	c.Assert(err, qt.IsNil)

	// Create a commodity registry
	commodityRegistry := boltdb.NewCommodityRegistry(db, areaRegistry, settingsRegistry)

	// Return the registries and a cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return commodityRegistry, areaRegistry, locationRegistry, cleanup
}

func getCommodityRegistry(t *testing.T) (registry.CommodityRegistry, *models.Commodity, func()) {
	c := qt.New(t)
	c.Helper()

	commodityRegistry, areaRegistry, locationRegistry, cleanup := setupTestCommodityRegistry(t)

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
	c.Assert(createdCommodity.Name, qt.Equals, "commodity1")
	c.Assert(createdCommodity.AreaID, qt.Equals, area1.ID)

	return commodityRegistry, createdCommodity, cleanup
}

func TestCommodityRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of CommodityRegistry
	r, _, cleanup := getCommodityRegistry(t)
	defer cleanup()

	// Verify the count of commodities in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityRegistry_AddImage(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of CommodityRegistry
	r, createdCommodity, cleanup := getCommodityRegistry(t)
	defer cleanup()

	// Add an image to the commodity
	err := r.AddImage(createdCommodity.ID, "image1")
	c.Assert(err, qt.IsNil)
	err = r.AddImage(createdCommodity.ID, "image2")
	c.Assert(err, qt.IsNil)

	// Get the images of the commodity
	images, err := r.GetImages(createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.Contains, "image1")
	c.Assert(images, qt.Contains, "image2")

	// Delete an image from the commodity
	err = r.DeleteImage(createdCommodity.ID, "image1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted image is not present in the commodity's images
	images, err = r.GetImages(createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.Not(qt.Contains), "image1")
	c.Assert(images, qt.Contains, "image2")
}

func TestCommodityRegistry_AddManual(t *testing.T) {
	c := qt.New(t)

	r, createdCommodity, cleanup := getCommodityRegistry(t)
	defer cleanup()

	// Add a manual to the commodity
	err := r.AddManual(createdCommodity.ID, "manual1")
	c.Assert(err, qt.IsNil)
	err = r.AddManual(createdCommodity.ID, "manual2")
	c.Assert(err, qt.IsNil)

	// Get the manuals of the commodity
	manuals, err := r.GetManuals(createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.Contains, "manual1")
	c.Assert(manuals, qt.Contains, "manual2")

	// Delete a manual from the commodity
	err = r.DeleteManual(createdCommodity.ID, "manual1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted manual is not present in the commodity's manuals
	manuals, err = r.GetManuals(createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.Not(qt.Contains), "manual1")
	c.Assert(manuals, qt.Contains, "manual2")
}

func TestCommodityRegistry_AddInvoice(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of CommodityRegistry
	r, createdCommodity, cleanup := getCommodityRegistry(t)
	defer cleanup()

	// Add an invoice to the commodity
	err := r.AddInvoice(createdCommodity.ID, "invoice1")
	c.Assert(err, qt.IsNil)
	err = r.AddInvoice(createdCommodity.ID, "invoice2")
	c.Assert(err, qt.IsNil)

	// Get the invoices for the commodity
	invoices, err := r.GetInvoices(createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.Contains, "invoice1")
	c.Assert(invoices, qt.Contains, "invoice2")

	// Delete an invoice from the commodity
	err = r.DeleteInvoice(createdCommodity.ID, "invoice1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted invoice is not present in the commodity's invoices
	invoices, err = r.GetInvoices(createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.Not(qt.Contains), "invoice1")
	c.Assert(invoices, qt.Contains, "invoice2")
}

func TestCommodityRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of CommodityRegistry
	r, createdCommodity, cleanup := getCommodityRegistry(t)
	defer cleanup()

	// Delete the commodity from the registry
	err := r.Delete(createdCommodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the commodity is no longer present in the registry
	_, err = r.Get(createdCommodity.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of commodities in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestCommodityRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of CommodityRegistry
	commodityRegistry, _, _, cleanup := setupTestCommodityRegistry(t)
	defer cleanup()

	// Create a test commodity without required fields
	commodity := models.Commodity{}

	// Attempt to create the commodity in the registry and expect a validation error
	_, err := commodityRegistry.Create(commodity)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Contains, "name")
}

func TestCommodityRegistry_Create_AreaNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of CommodityRegistry
	commodityRegistry, _, _, cleanup := setupTestCommodityRegistry(t)
	defer cleanup()

	// Create a test commodity with an invalid area ID
	commodity := models.Commodity{
		AreaID:    "invalid",
		Name:      "test",
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeEquipment,
		Count:     1,
		ShortName: "test",
	}

	// Attempt to create the commodity in the registry and expect an area not found error
	_, err := commodityRegistry.Create(commodity)
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestCommodityRegistry_Delete_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of CommodityRegistry
	commodityRegistry, _, _, cleanup := setupTestCommodityRegistry(t)
	defer cleanup()

	// Attempt to delete a non-existing commodity from the registry and expect a not found error
	err := commodityRegistry.Delete("nonexistent")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
