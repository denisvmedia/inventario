package registry_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func TestEntityService_DeleteCommodityRecursive(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registry set with proper dependencies
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)

	// Create entity service
	entityService := services.NewEntityService(registrySet)

	// Create test data hierarchy: Location -> Area -> Commodity -> Files
	location := models.Location{Name: "Test Location"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{Name: "Test Area", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		Name:   "Test Commodity",
		AreaID: createdArea.ID,
		Type:   models.CommodityTypeElectronics,
		Status: models.CommodityStatusInUse,
		Count:  1,
	}
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Create test files linked to the commodity
	now := time.Now()

	// Create an image file
	imageFile := models.FileEntity{
		Title:            "Test Image",
		Description:      "A test image file",
		Type:             models.FileTypeImage,
		Tags:             []string{"test"},
		LinkedEntityType: "commodity",
		LinkedEntityID:   createdCommodity.ID,
		LinkedEntityMeta: "images",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "test-image",
			OriginalPath: "test-image.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}
	createdImageFile, err := registrySet.FileRegistry.Create(ctx, imageFile)
	c.Assert(err, qt.IsNil)

	// Create a manual file
	manualFile := models.FileEntity{
		Title:            "Test Manual",
		Description:      "A test manual file",
		Type:             models.FileTypeDocument,
		Tags:             []string{"test"},
		LinkedEntityType: "commodity",
		LinkedEntityID:   createdCommodity.ID,
		LinkedEntityMeta: "manuals",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "test-manual",
			OriginalPath: "test-manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	createdManualFile, err := registrySet.FileRegistry.Create(ctx, manualFile)
	c.Assert(err, qt.IsNil)

	// Create an invoice file
	invoiceFile := models.FileEntity{
		Title:            "Test Invoice",
		Description:      "A test invoice file",
		Type:             models.FileTypeDocument,
		Tags:             []string{"test"},
		LinkedEntityType: "commodity",
		LinkedEntityID:   createdCommodity.ID,
		LinkedEntityMeta: "invoices",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "test-invoice",
			OriginalPath: "test-invoice.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	createdInvoiceFile, err := registrySet.FileRegistry.Create(ctx, invoiceFile)
	c.Assert(err, qt.IsNil)

	// Verify all files exist and are linked to the commodity
	files, err := registrySet.FileRegistry.ListByLinkedEntity(ctx, "commodity", createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 3)

	// Verify each file type exists
	imageFiles, err := registrySet.FileRegistry.ListByLinkedEntityAndMeta(ctx, "commodity", createdCommodity.ID, "images")
	c.Assert(err, qt.IsNil)
	c.Assert(imageFiles, qt.HasLen, 1)
	c.Assert(imageFiles[0].ID, qt.Equals, createdImageFile.ID)

	manualFiles, err := registrySet.FileRegistry.ListByLinkedEntityAndMeta(ctx, "commodity", createdCommodity.ID, "manuals")
	c.Assert(err, qt.IsNil)
	c.Assert(manualFiles, qt.HasLen, 1)
	c.Assert(manualFiles[0].ID, qt.Equals, createdManualFile.ID)

	invoiceFiles, err := registrySet.FileRegistry.ListByLinkedEntityAndMeta(ctx, "commodity", createdCommodity.ID, "invoices")
	c.Assert(err, qt.IsNil)
	c.Assert(invoiceFiles, qt.HasLen, 1)
	c.Assert(invoiceFiles[0].ID, qt.Equals, createdInvoiceFile.ID)

	// Test recursive delete
	err = entityService.DeleteCommodityRecursive(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify commodity is deleted
	_, err = registrySet.CommodityRegistry.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	// Verify all linked files are deleted
	files, err = registrySet.FileRegistry.ListByLinkedEntity(ctx, "commodity", createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 0)

	// Verify individual files are deleted
	_, err = registrySet.FileRegistry.Get(ctx, createdImageFile.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	_, err = registrySet.FileRegistry.Get(ctx, createdManualFile.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	_, err = registrySet.FileRegistry.Get(ctx, createdInvoiceFile.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	// Verify area and location still exist
	_, err = registrySet.AreaRegistry.Get(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil) // Should still exist

	_, err = registrySet.LocationRegistry.Get(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNil) // Should still exist
}

func TestEntityService_DeleteCommodityRecursive_NoFiles(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registry set with proper dependencies
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)

	// Create entity service
	entityService := services.NewEntityService(registrySet)

	// Create test data hierarchy: Location -> Area -> Commodity (no files)
	location := models.Location{Name: "Test Location"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{Name: "Test Area", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		Name:   "Test Commodity",
		AreaID: createdArea.ID,
		Type:   models.CommodityTypeElectronics,
		Status: models.CommodityStatusInUse,
		Count:  1,
	}
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Verify no files are linked to the commodity
	files, err := registrySet.FileRegistry.ListByLinkedEntity(ctx, "commodity", createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 0)

	// Test recursive delete (should work even with no files)
	err = entityService.DeleteCommodityRecursive(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil)

	// Verify commodity is deleted
	_, err = registrySet.CommodityRegistry.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	// Verify area and location still exist
	_, err = registrySet.AreaRegistry.Get(ctx, createdArea.ID)
	c.Assert(err, qt.IsNil) // Should still exist

	_, err = registrySet.LocationRegistry.Get(ctx, createdLocation.ID)
	c.Assert(err, qt.IsNil) // Should still exist
}

func TestEntityService_DeleteCommodityRecursive_NonExistentCommodity(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registry set with proper dependencies
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	c.Assert(err, qt.IsNil)

	// Create entity service
	entityService := services.NewEntityService(registrySet)

	// Test recursive delete on non-existent commodity
	err = entityService.DeleteCommodityRecursive(ctx, "non-existent-id")
	c.Assert(err, qt.IsNotNil) // Should fail
}
