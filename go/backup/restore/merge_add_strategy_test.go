package restore_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"text/template"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/processor"
	"github.com/denisvmedia/inventario/backup/restore/types"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // Import blob drivers
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// TemplateData holds the IDs for the XML template
type TemplateData struct {
	LocationID  string
	AreaID      string
	CommodityID string
	ImageID     string
	InvoiceID   string
	ManualID    string
	NewImageID  string // For templates that include new files
}

// generateXMLFromTemplate creates XML using the specified template with provided IDs
func generateXMLFromTemplate(templateFile string, data TemplateData) (string, error) {
	templateContent, err := os.ReadFile(templateFile)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("inventory").Parse(string(templateContent))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func TestRestoreService_MergeAddStrategy_NoDuplicateFiles(t *testing.T) {
	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})

	// Create registry set with proper dependencies
	registrySet := memory.NewRegistrySet()
	c.Assert(registrySet, qt.IsNotNil)

	// Set up main currency in settings (required for commodity validation)
	mainCurrency := "USD"
	settings := models.SettingsObject{
		MainCurrency: &mainCurrency,
	}
	err := registrySet.SettingsRegistry.Save(ctx, settings)
	c.Assert(err, qt.IsNil)

	// Create restore service
	entityService := services.NewEntityService(registrySet, "file://./test_uploads?create_dir=true")
	proc := processor.NewRestoreOperationProcessor("test-restore-op", registrySet, entityService, "file://./test_uploads?create_dir=true")

	// Generate initial XML with hardcoded IDs (will be ignored due to security enhancement)
	initialData := TemplateData{
		LocationID:  "test-location-1",
		AreaID:      "test-area-1",
		CommodityID: "test-commodity-1",
		ImageID:     "test-image-1",
		InvoiceID:   "test-invoice-1",
		ManualID:    "test-manual-1",
	}

	initialXML, err := generateXMLFromTemplate("testdata/inventory_template.xml", initialData)
	c.Assert(err, qt.IsNil)

	// First restore with full replace to create initial data
	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader := strings.NewReader(initialXML)
	stats, err := proc.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0)

	// Verify initial data was created
	c.Assert(stats.LocationCount, qt.Equals, 1)
	c.Assert(stats.AreaCount, qt.Equals, 1)
	c.Assert(stats.CommodityCount, qt.Equals, 1)
	c.Assert(stats.ImageCount, qt.Equals, 1)
	c.Assert(stats.InvoiceCount, qt.Equals, 1)
	c.Assert(stats.ManualCount, qt.Equals, 1)

	// Get initial counts from database
	initialImages, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	initialImageCount := len(initialImages)

	initialInvoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	initialInvoiceCount := len(initialInvoices)

	initialManuals, err := registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	initialManualCount := len(initialManuals)

	c.Assert(initialImageCount, qt.Equals, 1)
	c.Assert(initialInvoiceCount, qt.Equals, 1)
	c.Assert(initialManualCount, qt.Equals, 1)

	// Get the actual database IDs that were generated during the first restore
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations), qt.Equals, 1)
	locationID := locations[0].ID

	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 1)
	areaID := areas[0].ID

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)
	commodityID := commodities[0].ID

	images, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(images), qt.Equals, 1)
	imageID := images[0].ID

	invoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invoices), qt.Equals, 1)
	invoiceID := invoices[0].ID

	manuals, err := registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(manuals), qt.Equals, 1)
	manualID := manuals[0].ID

	// Generate XML with real database IDs for the merge test
	realData := TemplateData{
		LocationID:  locationID,
		AreaID:      areaID,
		CommodityID: commodityID,
		ImageID:     imageID,
		InvoiceID:   invoiceID,
		ManualID:    manualID,
	}

	xmlWithRealIDs, err := generateXMLFromTemplate("testdata/inventory_template.xml", realData)
	c.Assert(err, qt.IsNil)

	// Now try to restore the XML with real database IDs using Merge & Add strategy
	// This should detect duplicates correctly and NOT create new entities
	mergeAddOptions := types.RestoreOptions{
		Strategy:        types.RestoreStrategyMergeAdd,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader2 := strings.NewReader(xmlWithRealIDs)
	stats2, err := proc.RestoreFromXML(ctx, reader2, mergeAddOptions)
	c.Assert(err, qt.IsNil)
	c.Assert(stats2.ErrorCount, qt.Equals, 0)

	// With XML containing real database IDs, merge should detect duplicates correctly
	c.Assert(stats2.CreatedCount, qt.Equals, 0, qt.Commentf("No new items should be created"))
	c.Assert(stats2.SkippedCount > 0, qt.IsTrue, qt.Commentf("Items should be skipped"))

	// Verify no new files were created
	c.Assert(stats2.ImageCount, qt.Equals, 0, qt.Commentf("No new images should be created"))
	c.Assert(stats2.InvoiceCount, qt.Equals, 0, qt.Commentf("No new invoices should be created"))
	c.Assert(stats2.ManualCount, qt.Equals, 0, qt.Commentf("No new manuals should be created"))

	// Verify database counts remain the same
	finalImages, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalImages), qt.Equals, initialImageCount, qt.Commentf("Image count should remain the same"))

	finalInvoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalInvoices), qt.Equals, initialInvoiceCount, qt.Commentf("Invoice count should remain the same"))

	finalManuals, err := registrySet.ManualRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalManuals), qt.Equals, initialManualCount, qt.Commentf("Manual count should remain the same"))
}

func TestRestoreService_MergeAddStrategy_AddNewFilesOnly(t *testing.T) {
	c := qt.New(t)
	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})

	// Create registry set with proper dependencies
	registrySet := memory.NewRegistrySet()
	c.Assert(registrySet, qt.IsNotNil)

	// Set up main currency in settings (required for commodity validation)
	mainCurrency := "USD"
	settings := models.SettingsObject{
		MainCurrency: &mainCurrency,
	}
	err := registrySet.SettingsRegistry.Save(ctx, settings)
	c.Assert(err, qt.IsNil)

	// Create restore service
	entityService := services.NewEntityService(registrySet, "file://./test_uploads?create_dir=true")
	proc := processor.NewRestoreOperationProcessor("test-restore-op", registrySet, entityService, "file://./test_uploads?create_dir=true")

	// Generate initial data with one file using template
	initialData := TemplateData{
		LocationID:  "test-location-1",
		AreaID:      "test-area-1",
		CommodityID: "test-commodity-1",
		ImageID:     "test-image-1",
		InvoiceID:   "test-invoice-1",
	}

	initialXML, err := generateXMLFromTemplate("testdata/inventory_simple_template.xml", initialData)
	c.Assert(err, qt.IsNil)

	// First restore with full replace to create initial data
	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader := strings.NewReader(initialXML)
	stats, err := proc.RestoreFromXML(ctx, reader, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0)
	c.Assert(stats.ImageCount, qt.Equals, 1)

	// Get initial counts
	initialImages, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	initialImageCount := len(initialImages)
	c.Assert(initialImageCount, qt.Equals, 1)

	// Extract real database IDs from the first restore
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations), qt.Equals, 1)
	locationID := locations[0].ID

	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 1)
	areaID := areas[0].ID

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)
	commodityID := commodities[0].ID

	images, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(images), qt.Equals, 1)
	imageID := images[0].ID

	invoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invoices), qt.Equals, 1)
	invoiceID := invoices[0].ID

	// Generate XML with real database IDs + new files using template
	newFilesData := TemplateData{
		LocationID:  locationID,
		AreaID:      areaID,
		CommodityID: commodityID,
		ImageID:     imageID,     // Existing image (should be skipped)
		InvoiceID:   invoiceID,   // Existing invoice (should be skipped)
		NewImageID:  "test-image-2", // New image (should be created)
	}

	xmlWithNewFiles, err := generateXMLFromTemplate("testdata/inventory_with_new_files_template.xml", newFilesData)
	c.Assert(err, qt.IsNil)

	// Restore with Merge & Add strategy
	mergeAddOptions := types.RestoreOptions{
		Strategy:        types.RestoreStrategyMergeAdd,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader2 := strings.NewReader(xmlWithNewFiles)
	stats2, err := proc.RestoreFromXML(ctx, reader2, mergeAddOptions)
	c.Assert(err, qt.IsNil)
	c.Assert(stats2.ErrorCount, qt.Equals, 0)

	// Should create only the new files, not duplicate existing ones
	c.Assert(stats2.ImageCount, qt.Equals, 1, qt.Commentf("Should create 1 new image (test-image-2)"))
	c.Assert(stats2.InvoiceCount, qt.Equals, 0, qt.Commentf("Should not create duplicate invoice"))

	// Verify final counts in database - with template using real database IDs, duplicates are detected correctly
	finalImages, err := registrySet.ImageRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalImages), qt.Equals, 2, qt.Commentf("Should have 2 images total (1 existing + 1 new from second restore)"))

	finalInvoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalInvoices), qt.Equals, 1, qt.Commentf("Should have 1 invoice total (existing one, no duplicates)"))

	// Verify that we have the expected unique image IDs (1 existing + 1 new)
	imageIDs := make(map[string]bool)
	for _, img := range finalImages {
		imageIDs[img.ID] = true
	}
	c.Assert(len(imageIDs), qt.Equals, 2, qt.Commentf("Should have 2 unique image IDs (1 existing + 1 new)"))
}
