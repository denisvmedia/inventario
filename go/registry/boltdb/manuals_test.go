package boltdb_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

func setupTestManualRegistry(t *testing.T) (*boltdb.ManualRegistry, *boltdb.CommodityRegistry, *boltdb.AreaRegistry, *boltdb.LocationRegistry, func()) {
	c := qt.New(t)

	// Create a temporary directory for the test database
	tempDir := c.TempDir()

	// Create a new database in the temporary directory
	db, err := dbx.NewDB(tempDir, "test.db").Open()
	c.Assert(err, qt.IsNil)

	// Create a location registry
	locationRegistry := boltdb.NewLocationRegistry(db)

	// Create an area registry
	areaRegistry := boltdb.NewAreaRegistry(db, locationRegistry)

	// Create a file registry
	fileRegistry := boltdb.NewFileRegistry(db)

	// Create a commodity registry
	commodityRegistry := boltdb.NewCommodityRegistry(db, areaRegistry, fileRegistry)

	// Create a manual registry
	manualRegistry := boltdb.NewManualRegistry(db, commodityRegistry)

	// Return the registries and a cleanup function
	cleanup := func() {
		err = db.Close()
		c.Assert(err, qt.IsNil)
	}

	return manualRegistry, commodityRegistry, areaRegistry, locationRegistry, cleanup
}

func getManualTestSetup(t *testing.T) (registry.ManualRegistry, *models.Commodity, func()) {
	c := qt.New(t)
	ctx := context.Background()

	manualRegistry, commodityRegistry, areaRegistry, locationRegistry, cleanup := setupTestManualRegistry(t)

	location1, err := locationRegistry.Create(ctx, models.Location{
		Name: "Location 1",
	})
	c.Assert(err, qt.IsNil)

	area1, err := areaRegistry.Create(ctx, models.Area{
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

	createdCommodity, err := commodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)
	c.Assert(createdCommodity, qt.Not(qt.IsNil))

	return manualRegistry, createdCommodity, cleanup
}

func TestManualRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ManualRegistry
	r, createdCommodity, cleanup := getManualTestSetup(t)
	defer cleanup()

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
	r, createdCommodity, cleanup := getManualTestSetup(t)
	defer cleanup()

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
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of manuals in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestManualRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ManualRegistry
	r, _, cleanup := getManualTestSetup(t)
	defer cleanup()

	// Create a test manual without required fields
	manual := models.Manual{}
	_, err := r.Create(ctx, manual)
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestManualRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of ManualRegistry
	r, _, cleanup := getManualTestSetup(t)
	defer cleanup()

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
	c.Assert(err, qt.Not(qt.IsNil))
}
