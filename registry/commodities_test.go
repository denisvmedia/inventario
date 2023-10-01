package registry_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestMemoryCommodityRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryCommodityRegistry
	r, _ := getCommodityRegistry(c) // will create the commodity

	// Verify the count of commodities in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestMemoryCommodityRegistry_AddImage(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryCommodityRegistry
	r, createdCommodity := getCommodityRegistry(c)

	// Add an image to the commodity
	r.AddImage(createdCommodity.ID, "image1")
	r.AddImage(createdCommodity.ID, "image2")

	// Get the images of the commodity
	images := r.GetImages(createdCommodity.ID)
	c.Assert(images, qt.Contains, "image1")
	c.Assert(images, qt.Contains, "image2")

	// Delete an image from the commodity
	r.DeleteImage(createdCommodity.ID, "image1")

	// Verify that the deleted image is not present in the commodity's images
	images = r.GetImages(createdCommodity.ID)
	c.Assert(images, qt.Not(qt.Contains), "image1")
	c.Assert(images, qt.Contains, "image2")
}

func TestMemoryCommodityRegistry_AddManual(t *testing.T) {
	c := qt.New(t)

	r, createdCommodity := getCommodityRegistry(c)

	// Add a manual to the commodity
	r.AddManual(createdCommodity.ID, "manual1")
	r.AddManual(createdCommodity.ID, "manual2")

	// Get the manuals of the commodity
	manuals := r.GetManuals(createdCommodity.ID)
	c.Assert(manuals, qt.Contains, "manual1")
	c.Assert(manuals, qt.Contains, "manual2")

	// Delete a manual from the commodity
	r.DeleteManual(createdCommodity.ID, "manual1")

	// Verify that the deleted manual is not present in the commodity's manuals
	manuals = r.GetManuals(createdCommodity.ID)
	c.Assert(manuals, qt.Not(qt.Contains), "manual1")
	c.Assert(manuals, qt.Contains, "manual2")
}

func TestMemoryCommodityRegistry_AddInvoice(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryCommodityRegistry
	r, createdCommodity := getCommodityRegistry(c)

	// Add an invoice to the commodity
	r.AddInvoice(createdCommodity.ID, "invoice1")
	r.AddInvoice(createdCommodity.ID, "invoice2")

	// Get the invoices for the commodity
	invoices := r.GetInvoices(createdCommodity.ID)
	c.Assert(invoices, qt.Contains, "invoice1")
	c.Assert(invoices, qt.Contains, "invoice2")

	// Delete an invoice from the commodity
	r.DeleteInvoice(createdCommodity.ID, "invoice1")

	// Verify that the deleted invoice is not present in the commodity's invoices
	invoices = r.GetInvoices(createdCommodity.ID)
	c.Assert(invoices, qt.Not(qt.Contains), "invoice1")
	c.Assert(invoices, qt.Contains, "invoice2")
}

func TestMemoryCommodityRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryCommodityRegistry
	r, createdCommodity := getCommodityRegistry(c)

	// Delete the commodity from the registry
	err := r.Delete(createdCommodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the commodity is no longer present in the registry
	_, err = r.Get(createdCommodity.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of commodities in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestMemoryCommodityRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryCommodityRegistry
	locationRegistry := registry.NewMemoryLocationRegistry()
	areaRegistry := registry.NewMemoryAreaRegistry(locationRegistry)
	r := registry.NewMemoryCommodityRegistry(areaRegistry)

	// Create a test commodity without required fields
	commodity := models.Commodity{}

	// Attempt to create the commodity in the registry and expect a validation error
	_, err := r.Create(commodity)
	var errs validation.Errors
	c.Assert(err, qt.ErrorAs, &errs)
	c.Assert(errs, qt.HasLen, 6)
	c.Assert(errs["name"], qt.Not(qt.IsNil))
	c.Assert(errs["name"].Error(), qt.Equals, "cannot be blank")
	c.Assert(errs["short_name"], qt.Not(qt.IsNil))
	c.Assert(errs["short_name"].Error(), qt.Equals, "cannot be blank")
	c.Assert(errs["type"], qt.Not(qt.IsNil))
	c.Assert(errs["type"].Error(), qt.Equals, "cannot be blank")
	c.Assert(errs["area_id"], qt.Not(qt.IsNil))
	c.Assert(errs["area_id"].Error(), qt.Equals, "cannot be blank")
	c.Assert(errs["status"], qt.Not(qt.IsNil))
	c.Assert(errs["status"].Error(), qt.Equals, "cannot be blank")
	c.Assert(errs["count"], qt.Not(qt.IsNil))
	c.Assert(errs["count"].Error(), qt.Equals, "cannot be blank")
}

func TestMemoryCommodityRegistry_Create_AreaNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryCommodityRegistry
	locationRegistry := registry.NewMemoryLocationRegistry()
	areaRegistry := registry.NewMemoryAreaRegistry(locationRegistry)
	r := registry.NewMemoryCommodityRegistry(areaRegistry)

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
	_, err := r.Create(commodity)
	c.Assert(err, qt.ErrorMatches, "area not found.*")
}

func TestMemoryCommodityRegistry_Delete_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryCommodityRegistry
	locationRegistry := registry.NewMemoryLocationRegistry()
	areaRegistry := registry.NewMemoryAreaRegistry(locationRegistry)
	r := registry.NewMemoryCommodityRegistry(areaRegistry)

	// Attempt to delete a non-existing commodity from the registry and expect a not found error
	err := r.Delete("nonexistent")
	c.Assert(err, qt.ErrorMatches, "not found.*")
}

func getCommodityRegistry(c *qt.C) (registry.CommodityRegistry, *models.Commodity) {
	locationRegistry := registry.NewMemoryLocationRegistry()
	areaRegistry := registry.NewMemoryAreaRegistry(locationRegistry)
	r := registry.NewMemoryCommodityRegistry(areaRegistry)

	location1, err := locationRegistry.Create(models.Location{
		Name: "Location 1",
	})
	c.Assert(err, qt.IsNil)

	area1, err := areaRegistry.Create(models.Area{
		Name:       "Area 1",
		LocationID: location1.ID,
	})
	c.Assert(err, qt.IsNil)

	createdCommodity, err := r.Create(*models.WithID("commodity1", &models.Commodity{
		AreaID:    area1.ID,
		Name:      "commodity1",
		ShortName: "commodity1",
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeWhiteGoods,
		Count:     1,
	}))
	c.Assert(err, qt.IsNil)
	c.Assert(createdCommodity, qt.Not(qt.IsNil))
	c.Assert(createdCommodity.ID, qt.Not(qt.Equals), "commodity1")
	c.Assert(createdCommodity.Name, qt.Equals, "commodity1")
	c.Assert(createdCommodity.AreaID, qt.Equals, area1.ID)

	return r, createdCommodity
}
