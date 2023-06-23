package registry_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestMemoryManualRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryManualRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := registry.NewMemoryManualRegistry(commodityRegistry)

	// Create a test manual
	manual := models.Manual{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Create a new manual in the registry
	createdManual, err := r.Create(manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.Not(qt.IsNil))

	// Verify the count of manuals in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestMemoryManualRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryManualRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := registry.NewMemoryManualRegistry(commodityRegistry)

	// Create a test manual
	manual := models.Manual{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Create a new manual in the registry
	createdManual, err := r.Create(manual)
	c.Assert(err, qt.IsNil)

	// Delete the manual from the registry
	err = r.Delete(createdManual.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the manual is no longer present in the registry
	_, err = r.Get(createdManual.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of manuals in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestMemoryManualRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryManualRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := registry.NewMemoryManualRegistry(commodityRegistry)

	// Create a test manual without required fields
	manual := models.Manual{}
	_, err := r.Create(manual)
	c.Assert(err, qt.Not(qt.IsNil))
	var errs validation.Errors
	c.Assert(err, qt.ErrorAs, &errs)
	c.Assert(errs["File"], qt.Not(qt.IsNil))
	c.Assert(errs["File"].Error(), qt.Equals, "cannot be blank")
	c.Assert(errs["commodity_id"], qt.Not(qt.IsNil))
	c.Assert(errs["commodity_id"].Error(), qt.Equals, "cannot be blank")

	manual = models.Manual{
		File: &models.File{
			Path:     "test",
			Ext:      ".png",
			MIMEType: "image/png",
		},
		CommodityID: "invalid",
	}
	// Attempt to create the manual in the registry and expect a validation error
	_, err = r.Create(manual)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}

func TestMemoryManualRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryManualRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := registry.NewMemoryManualRegistry(commodityRegistry)

	// Create a test manual with an invalid commodity ID
	manual := models.Manual{
		CommodityID: "invalid",
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Attempt to create the manual in the registry and expect a commodity not found error
	_, err := r.Create(manual)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}
