package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestImageRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ImageRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := memory.NewImageRegistry(commodityRegistry)

	// Create a test image
	image := models.Image{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create a new image in the registry
	createdImage, err := r.Create(ctx, image)
	c.Assert(err, qt.IsNil)
	c.Assert(createdImage, qt.Not(qt.IsNil))

	// Verify the count of images in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestImageRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ImageRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := memory.NewImageRegistry(commodityRegistry)

	// Create a test image
	image := models.Image{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create a new image in the registry
	createdImage, err := r.Create(ctx, image)
	c.Assert(err, qt.IsNil)

	// Delete the image from the registry
	err = r.Delete(ctx, createdImage.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the image is no longer present in the registry
	_, err = r.Get(ctx, createdImage.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of images in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestImageRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ImageRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := memory.NewImageRegistry(commodityRegistry)

	// Create a test image without required fields
	image := models.Image{}
	_, err := r.Create(ctx, image)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err, qt.ErrorMatches, "commodity not found:.*")

	image = models.Image{
		File: &models.File{
			Path:         "test", // Without extension
			OriginalPath: "test.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
		CommodityID: "invalid",
	}
	// Attempt to create the image in the registry and expect a validation error
	_, err = r.Create(ctx, image)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}

func TestImageRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ImageRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := memory.NewImageRegistry(commodityRegistry)

	// Create a test image with an invalid commodity ID
	image := models.Image{
		CommodityID: "invalid",
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Attempt to create the image in the registry and expect a commodity not found error
	_, err := r.Create(ctx, image)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}
