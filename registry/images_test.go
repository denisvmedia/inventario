package registry_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestMemoryImageRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryImageRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := registry.NewMemoryImageRegistry(commodityRegistry)

	// Create a test image
	image := models.Image{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Create a new image in the registry
	createdImage, err := r.Create(image)
	c.Assert(err, qt.IsNil)
	c.Assert(createdImage, qt.Not(qt.IsNil))

	// Verify the count of images in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestMemoryImageRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryImageRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := registry.NewMemoryImageRegistry(commodityRegistry)

	// Create a test image
	image := models.Image{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Create a new image in the registry
	createdImage, err := r.Create(image)
	c.Assert(err, qt.IsNil)

	// Delete the image from the registry
	err = r.Delete(createdImage.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the image is no longer present in the registry
	_, err = r.Get(createdImage.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of images in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestMemoryImageRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryImageRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := registry.NewMemoryImageRegistry(commodityRegistry)

	// Create a test image without required fields
	image := models.Image{}
	_, err := r.Create(image)
	c.Assert(err, qt.Not(qt.IsNil))
	var errs validation.Errors
	c.Assert(err, qt.ErrorAs, &errs)
	c.Assert(errs["File"], qt.Not(qt.IsNil))
	c.Assert(errs["File"].Error(), qt.Equals, "cannot be blank")
	c.Assert(errs["commodity_id"], qt.Not(qt.IsNil))
	c.Assert(errs["commodity_id"].Error(), qt.Equals, "cannot be blank")

	image = models.Image{
		File: &models.File{
			Path:     "test",
			Ext:      ".png",
			MIMEType: "image/png",
		},
		CommodityID: "invalid",
	}
	// Attempt to create the image in the registry and expect a validation error
	_, err = r.Create(image)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}

func TestMemoryImageRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of MemoryImageRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := registry.NewMemoryImageRegistry(commodityRegistry)

	// Create a test image with an invalid commodity ID
	image := models.Image{
		CommodityID: "invalid",
		File: &models.File{
			Path:     "path",
			Ext:      ".ext",
			MIMEType: "octet/stream",
		},
	}

	// Attempt to create the image in the registry and expect a commodity not found error
	_, err := r.Create(image)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}
