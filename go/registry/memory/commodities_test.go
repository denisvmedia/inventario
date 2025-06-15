package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestCommodityRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	r, _ := getCommodityRegistry(c) // will create the commodity

	// Verify the count of commodities in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityRegistry_AddImage(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	r, createdCommodity := getCommodityRegistry(c)

	// Add an image to the commodity
	err := r.AddImage(ctx, createdCommodity.ID, "image1")
	c.Assert(err, qt.IsNil)
	err = r.AddImage(ctx, createdCommodity.ID, "image2")
	c.Assert(err, qt.IsNil)

	// Get the images of the commodity
	images, err := r.GetImages(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.Contains, "image1")
	c.Assert(images, qt.Contains, "image2")

	// Delete an image from the commodity
	err = r.DeleteImage(ctx, createdCommodity.ID, "image1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted image is not present in the commodity's images
	images, err = r.GetImages(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.Not(qt.Contains), "image1")
	c.Assert(images, qt.Contains, "image2")
}

func TestCommodityRegistry_AddManual(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r, createdCommodity := getCommodityRegistry(c)

	// Add a manual to the commodity
	err := r.AddManual(ctx, createdCommodity.ID, "manual1")
	c.Assert(err, qt.IsNil)
	err = r.AddManual(ctx, createdCommodity.ID, "manual2")
	c.Assert(err, qt.IsNil)

	// Get the manuals of the commodity
	manuals, err := r.GetManuals(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.Contains, "manual1")
	c.Assert(manuals, qt.Contains, "manual2")

	// Delete a manual from the commodity
	err = r.DeleteManual(ctx, createdCommodity.ID, "manual1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted manual is not present in the commodity's manuals
	manuals, err = r.GetManuals(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.Not(qt.Contains), "manual1")
	c.Assert(manuals, qt.Contains, "manual2")
}

func TestCommodityRegistry_AddInvoice(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	r, createdCommodity := getCommodityRegistry(c)

	// Add an invoice to the commodity
	err := r.AddInvoice(ctx, createdCommodity.ID, "invoice1")
	c.Assert(err, qt.IsNil)
	err = r.AddInvoice(ctx, createdCommodity.ID, "invoice2")
	c.Assert(err, qt.IsNil)

	// Get the invoices for the commodity
	invoices, err := r.GetInvoices(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.Contains, "invoice1")
	c.Assert(invoices, qt.Contains, "invoice2")

	// Delete an invoice from the commodity
	err = r.DeleteInvoice(ctx, createdCommodity.ID, "invoice1")
	c.Assert(err, qt.IsNil)

	// Verify that the deleted invoice is not present in the commodity's invoices
	invoices, err = r.GetInvoices(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.Not(qt.Contains), "invoice1")
	c.Assert(invoices, qt.Contains, "invoice2")
}

func TestCommodityRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	r, createdCommodity := getCommodityRegistry(c)

	// Delete the commodity from the registry
	err := r.Delete(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the commodity is no longer present in the registry
	_, err = r.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of commodities in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestCommodityRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)

	r := memory.NewCommodityRegistry(areaRegistry)

	// Create a test commodity without required fields
	commodity := models.Commodity{}

	// Attempt to create the commodity in the registry and expect a validation error
	_, err := r.Create(ctx, commodity)
	var errVal *errkit.Error
	c.Assert(err, qt.ErrorAs, &errVal)
	c.Assert(err.Error(), qt.Contains, "area not found: not found")
}

func TestCommodityRegistry_Create_AreaNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)

	r := memory.NewCommodityRegistry(areaRegistry)

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
	_, err := r.Create(ctx, commodity)
	c.Assert(err, qt.ErrorMatches, "area not found.*")
}

func TestCommodityRegistry_Delete_CommodityNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of CommodityRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	r := memory.NewCommodityRegistry(areaRegistry)

	// Attempt to delete a non-existing commodity from the registry and expect a not found error
	err := r.Delete(ctx, "nonexistent")
	c.Assert(err, qt.ErrorMatches, "failed to get commodity: not found")
}

func getCommodityRegistry(c *qt.C) (registry.CommodityRegistry, *models.Commodity) {
	ctx := context.Background()
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)

	r := memory.NewCommodityRegistry(areaRegistry)

	location1, err := locationRegistry.Create(ctx, models.Location{
		Name: "Location 1",
	})
	c.Assert(err, qt.IsNil)

	area1, err := areaRegistry.Create(ctx, models.Area{
		Name:       "Area 1",
		LocationID: location1.ID,
	})
	c.Assert(err, qt.IsNil)

	createdCommodity, err := r.Create(ctx, *models.WithID("commodity1", &models.Commodity{
		AreaID:    area1.ID,
		Name:      "commodity1",
		ShortName: "commodity1",
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeWhiteGoods,
		Count:     1,
	}))
	c.Assert(err, qt.IsNil)
	c.Assert(createdCommodity, qt.Not(qt.IsNil))
	c.Assert(createdCommodity.ID, qt.Equals, "commodity1")
	c.Assert(createdCommodity.Name, qt.Equals, "commodity1")
	c.Assert(createdCommodity.AreaID, qt.Equals, area1.ID)

	return r, createdCommodity
}
