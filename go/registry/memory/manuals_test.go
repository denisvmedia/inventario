package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestManualRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ManualRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := memory.NewManualRegistry(commodityRegistry)

	// Create a test manual
	manual := models.Manual{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create a new manual in the registry
	createdManual, err := r.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.Not(qt.IsNil))

	// Verify the count of manuals in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestManualRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ManualRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	r := memory.NewManualRegistry(commodityRegistry)

	// Create a test manual
	manual := models.Manual{
		CommodityID: createdCommodity.GetID(),
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Create a new manual in the registry
	createdManual, err := r.Create(ctx, manual)
	c.Assert(err, qt.IsNil)

	// Delete the manual from the registry
	err = r.Delete(ctx, createdManual.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the manual is no longer present in the registry
	_, err = r.Get(ctx, createdManual.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Verify the count of manuals in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestManualRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ManualRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := memory.NewManualRegistry(commodityRegistry)

	// Create a test manual without required fields
	manual := models.Manual{}
	_, err := r.Create(ctx, manual)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err, qt.ErrorMatches, "commodity not found:.*")

	manual = models.Manual{
		File: &models.File{
			Path:         "test", // Without extension
			OriginalPath: "test.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
		CommodityID: "invalid",
	}
	// Attempt to create the manual in the registry and expect a validation error
	_, err = r.Create(ctx, manual)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}

func TestManualRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ManualRegistry
	commodityRegistry, _ := getCommodityRegistry(c)
	r := memory.NewManualRegistry(commodityRegistry)

	// Create a test manual with an invalid commodity ID
	manual := models.Manual{
		CommodityID: "invalid",
		File: &models.File{
			Path:         "path", // Without extension
			OriginalPath: "path.ext",
			Ext:          ".ext",
			MIMEType:     "octet/stream",
		},
	}

	// Attempt to create the manual in the registry and expect a commodity not found error
	_, err := r.Create(ctx, manual)
	c.Assert(err, qt.ErrorMatches, "commodity not found.*")
}
