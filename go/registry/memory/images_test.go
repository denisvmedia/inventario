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

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create a new instance of ImageRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	baseRegistry := memory.NewImageRegistry(commodityRegistry)
	r, err := baseRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create a new instance of ImageRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	baseRegistry := memory.NewImageRegistry(commodityRegistry)
	r, err := baseRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create a new instance of ImageRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	baseImageRegistry := memory.NewImageRegistry(commodityRegistry)
	r, err := baseImageRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create a test image without required fields
	image := models.Image{}
	createdImage, err := r.Create(ctx, image)
	c.Assert(err, qt.IsNil)
	c.Assert(createdImage, qt.Not(qt.IsNil))

	image = models.Image{
		File: &models.File{
			Path:         "test", // Without extension
			OriginalPath: "test.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
		CommodityID: "invalid",
	}
	// Create the image - should succeed (no validation in memory registry)
	createdImage2, err := r.Create(ctx, image)
	c.Assert(err, qt.IsNil)
	c.Assert(createdImage2, qt.Not(qt.IsNil))
}

func TestImageRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx = registry.WithUserContext(ctx, userID)

	// Create a new instance of ImageRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	baseImageRegistry := memory.NewImageRegistry(commodityRegistry)
	r, err := baseImageRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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

	// Create the image - should succeed (no validation in memory registry)
	createdImage, err := r.Create(ctx, image)
	c.Assert(err, qt.IsNil)
	c.Assert(createdImage, qt.Not(qt.IsNil))
}
