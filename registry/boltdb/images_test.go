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

func setupTestImageRegistry(t *testing.T) (*boltdb.ImageRegistry, *boltdb.CommodityRegistry, *boltdb.AreaRegistry, *boltdb.LocationRegistry, func()) {
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

	// Create a commodity registry
	commodityRegistry := boltdb.NewCommodityRegistry(db, areaRegistry)

	// Create an image registry
	imageRegistry := boltdb.NewImageRegistry(db, commodityRegistry)

	// Return the registries and a cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return imageRegistry, commodityRegistry, areaRegistry, locationRegistry, cleanup
}

func getImageTestSetup(t *testing.T) (registry.ImageRegistry, *models.Commodity, func()) {
	c := qt.New(t)

	imageRegistry, commodityRegistry, areaRegistry, locationRegistry, cleanup := setupTestImageRegistry(t)

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

	return imageRegistry, createdCommodity, cleanup
}

func TestImageRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of ImageRegistry
	r, createdCommodity, cleanup := getImageTestSetup(t)
	defer cleanup()

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
	createdImage, err := r.Create(image)
	c.Assert(err, qt.IsNil)
	c.Assert(createdImage, qt.Not(qt.IsNil))

	// Verify the count of images in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestImageRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of ImageRegistry
	r, createdCommodity, cleanup := getImageTestSetup(t)
	defer cleanup()

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
	createdImage, err := r.Create(image)
	c.Assert(err, qt.IsNil)

	// Delete the image from the registry
	err = r.Delete(createdImage.ID)
	c.Assert(err, qt.IsNil)

	// Verify that the image is no longer present in the registry
	_, err = r.Get(createdImage.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify the count of images in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestImageRegistry_Create_Validation(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of ImageRegistry
	r, _, cleanup := getImageTestSetup(t)
	defer cleanup()

	// Create a test image without required fields
	image := models.Image{}
	_, err := r.Create(image)
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestImageRegistry_Create_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of ImageRegistry
	r, _, cleanup := getImageTestSetup(t)
	defer cleanup()

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
	_, err := r.Create(image)
	c.Assert(err, qt.Not(qt.IsNil))
}
