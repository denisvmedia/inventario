package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestManualRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
	})

	// Create a new instance of ManualRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	baseRegistry := memory.NewManualRegistry(commodityRegistry)
	r, err := baseRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
	})

	// Create a new instance of ManualRegistry
	commodityRegistry, createdCommodity := getCommodityRegistry(c)
	baseRegistry := memory.NewManualRegistry(commodityRegistry)
	r, err := baseRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
	})

	// Create a new instance of ManualRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	baseManualRegistry := memory.NewManualRegistry(commodityRegistry)
	r, err := baseManualRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	// Create a test manual without required fields
	manual := models.Manual{}
	createdManual, err := r.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.Not(qt.IsNil))

	manual = models.Manual{
		File: &models.File{
			Path:         "test", // Without extension
			OriginalPath: "test.png",
			Ext:          ".png",
			MIMEType:     "image/png",
		},
		CommodityID: "invalid",
	}
	// Create the manual - should succeed (no validation in memory registry)
	createdManual2, err := r.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual2, qt.Not(qt.IsNil))
}

func TestManualRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Add user context for user-aware entities
	userID := "test-user-123"
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: "test-tenant-id",
		},
	})

	// Create a new instance of ManualRegistry
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	baseManualRegistry := memory.NewManualRegistry(commodityRegistry)
	r, err := baseManualRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

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

	// Create the manual - should succeed (no validation in memory registry)
	createdManual, err := r.Create(ctx, manual)
	c.Assert(err, qt.IsNil)
	c.Assert(createdManual, qt.Not(qt.IsNil))
}
