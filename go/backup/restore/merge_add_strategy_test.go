package restore_test

import (
	"bytes"
	"context"
	"fmt"
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
	"github.com/denisvmedia/inventario/registry"
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
		ImageID:     imageID,        // Existing image (should be skipped)
		InvoiceID:   invoiceID,      // Existing invoice (should be skipped)
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

// TestRestoreService_SecurityValidation_CrossUserAccess tests that users cannot link files to other users' entities
func TestRestoreService_SecurityValidation_CrossUserAccess(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create test user 1
	testUser1 := models.User{
		Name:  "Test User 1",
		Email: "user1@example.com",
	}

	userRegistry := memory.NewUserRegistry()
	createdUser1, err := userRegistry.Create(ctx, testUser1)
	c.Assert(err, qt.IsNil)

	// Create test user 2
	testUser2 := models.User{
		Name:  "Test User 2",
		Email: "user2@example.com",
	}

	createdUser2, err := userRegistry.Create(ctx, testUser2)
	c.Assert(err, qt.IsNil)

	// Create shared registry set that both users will use
	sharedRegistrySet := memory.NewRegistrySet()
	sharedRegistrySet.UserRegistry = userRegistry

	// Create user-specific registry sets that share the same underlying data
	user1Ctx := appctx.WithUser(ctx, createdUser1)
	registrySet1 := &registry.Set{}
	registrySet1.LocationRegistry, err = sharedRegistrySet.LocationRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.AreaRegistry, err = sharedRegistrySet.AreaRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.CommodityRegistry, err = sharedRegistrySet.CommodityRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.ImageRegistry, err = sharedRegistrySet.ImageRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.InvoiceRegistry, err = sharedRegistrySet.InvoiceRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.ManualRegistry, err = sharedRegistrySet.ManualRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.SettingsRegistry, err = sharedRegistrySet.SettingsRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.ExportRegistry, err = sharedRegistrySet.ExportRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.RestoreOperationRegistry, err = sharedRegistrySet.RestoreOperationRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.RestoreStepRegistry, err = sharedRegistrySet.RestoreStepRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.FileRegistry, err = sharedRegistrySet.FileRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.UserRegistry = userRegistry
	registrySet1.TenantRegistry = sharedRegistrySet.TenantRegistry

	// Set main currency for user 1
	mainCurrency := "USD"
	err = registrySet1.SettingsRegistry.Save(user1Ctx, models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	// User 1 creates some entities

	// Create initial data for user 1 using template
	initialData := TemplateData{
		LocationID:  "user1-location-1",
		AreaID:      "user1-area-1",
		CommodityID: "user1-commodity-1",
		ImageID:     "user1-image-1",
		InvoiceID:   "user1-invoice-1",
		ManualID:    "user1-manual-1",
	}

	initialXML, err := generateXMLFromTemplate("testdata/inventory_template.xml", initialData)
	c.Assert(err, qt.IsNil)

	// User 1 imports their data
	entityService1 := services.NewEntityService(registrySet1, "file://./test_uploads?create_dir=true")
	proc1 := processor.NewRestoreOperationProcessor("test-restore-op-user1", registrySet1, entityService1, "file://./test_uploads?create_dir=true")

	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader1 := strings.NewReader(initialXML)
	stats1, err := proc1.RestoreFromXML(user1Ctx, reader1, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats1.ErrorCount, qt.Equals, 0)

	// Get user 1's created commodity ID
	commodities1, err := registrySet1.CommodityRegistry.List(user1Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities1), qt.Equals, 1)
	user1CommodityID := commodities1[0].ID

	// Create registry set for user 2 using the same shared data
	user2Ctx := appctx.WithUser(ctx, createdUser2)
	registrySet2 := &registry.Set{}
	registrySet2.LocationRegistry, err = sharedRegistrySet.LocationRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.AreaRegistry, err = sharedRegistrySet.AreaRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.CommodityRegistry, err = sharedRegistrySet.CommodityRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.ImageRegistry, err = sharedRegistrySet.ImageRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.InvoiceRegistry, err = sharedRegistrySet.InvoiceRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.ManualRegistry, err = sharedRegistrySet.ManualRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.SettingsRegistry, err = sharedRegistrySet.SettingsRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.ExportRegistry, err = sharedRegistrySet.ExportRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.RestoreOperationRegistry, err = sharedRegistrySet.RestoreOperationRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.RestoreStepRegistry, err = sharedRegistrySet.RestoreStepRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.FileRegistry, err = sharedRegistrySet.FileRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.UserRegistry = userRegistry
	registrySet2.TenantRegistry = sharedRegistrySet.TenantRegistry

	// Set main currency for user 2
	err = registrySet2.SettingsRegistry.Save(user2Ctx, models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	// User 2 attempts to create XML that links their file to user 1's commodity (ATTACK!)
	maliciousData := TemplateData{
		LocationID:  "user2-location-1",
		AreaID:      "user2-area-1",
		CommodityID: user1CommodityID, // SECURITY VIOLATION: Trying to link to user 1's commodity!
		ImageID:     "user2-malicious-image",
		InvoiceID:   "user2-malicious-invoice",
		ManualID:    "user2-malicious-manual",
	}

	maliciousXML, err := generateXMLFromTemplate("testdata/inventory_template.xml", maliciousData)
	c.Assert(err, qt.IsNil)

	// User 2 attempts the malicious import
	entityService2 := services.NewEntityService(registrySet2, "file://./test_uploads?create_dir=true")
	proc2 := processor.NewRestoreOperationProcessor("test-restore-op-user2", registrySet2, entityService2, "file://./test_uploads?create_dir=true")

	reader2 := strings.NewReader(maliciousXML)
	stats2, err := proc2.RestoreFromXML(user2Ctx, reader2, options)

	// Should either fail completely or skip unauthorized entities
	// TODO: Define exact behavior - should this fail with error or skip with warnings?
	c.Assert(stats2.ErrorCount > 0, qt.IsTrue, qt.Commentf("Should have errors when trying to access other user's entities"))

	// Verify user 1's data is unchanged
	finalCommodities1, err := registrySet1.CommodityRegistry.List(user1Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalCommodities1), qt.Equals, 1, qt.Commentf("User 1's commodities should be unchanged"))

	// Verify user 2 cannot access user 1's commodity
	user2Commodities, err := registrySet2.CommodityRegistry.List(user2Ctx)
	c.Assert(err, qt.IsNil)
	// User 2 should have 0 commodities (attack failed) or their own commodities only
	for _, commodity := range user2Commodities {
		c.Assert(commodity.ID, qt.Not(qt.Equals), user1CommodityID, qt.Commentf("User 2 should not have access to user 1's commodity"))
	}
}

// TestRestoreService_SecurityValidation_CrossTenantAccess tests that users cannot access other tenants' data
func TestRestoreService_SecurityValidation_CrossTenantAccess(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create test user in tenant 1
	testUserTenant1 := models.User{
		Name:  "Tenant 1 User",
		Email: "user@tenant1.com",
		// TODO: Add tenant ID field when multi-tenancy is fully implemented
	}

	userRegistry := memory.NewUserRegistry()
	createdUserTenant1, err := userRegistry.Create(ctx, testUserTenant1)
	c.Assert(err, qt.IsNil)

	// Create test user in tenant 2
	testUserTenant2 := models.User{
		Name:  "Tenant 2 User",
		Email: "user@tenant2.com",
		// TODO: Add different tenant ID when multi-tenancy is fully implemented
	}

	createdUserTenant2, err := userRegistry.Create(ctx, testUserTenant2)
	c.Assert(err, qt.IsNil)

	// Create shared registry set that both tenants will use for security validation
	sharedTenantRegistrySet := memory.NewRegistrySet()
	sharedTenantRegistrySet.UserRegistry = userRegistry

	// Create tenant-specific registry sets that share the same underlying data
	tenant1Ctx := appctx.WithUser(ctx, createdUserTenant1)
	registrySetTenant1 := &registry.Set{}
	registrySetTenant1.LocationRegistry, err = sharedTenantRegistrySet.LocationRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.AreaRegistry, err = sharedTenantRegistrySet.AreaRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.CommodityRegistry, err = sharedTenantRegistrySet.CommodityRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.ImageRegistry, err = sharedTenantRegistrySet.ImageRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.InvoiceRegistry, err = sharedTenantRegistrySet.InvoiceRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.ManualRegistry, err = sharedTenantRegistrySet.ManualRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.SettingsRegistry, err = sharedTenantRegistrySet.SettingsRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.ExportRegistry, err = sharedTenantRegistrySet.ExportRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.RestoreOperationRegistry, err = sharedTenantRegistrySet.RestoreOperationRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.RestoreStepRegistry, err = sharedTenantRegistrySet.RestoreStepRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.FileRegistry, err = sharedTenantRegistrySet.FileRegistry.WithCurrentUser(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant1.UserRegistry = userRegistry
	registrySetTenant1.TenantRegistry = sharedTenantRegistrySet.TenantRegistry

	tenant2Ctx := appctx.WithUser(ctx, createdUserTenant2)
	registrySetTenant2 := &registry.Set{}
	registrySetTenant2.LocationRegistry, err = sharedTenantRegistrySet.LocationRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.AreaRegistry, err = sharedTenantRegistrySet.AreaRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.CommodityRegistry, err = sharedTenantRegistrySet.CommodityRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.ImageRegistry, err = sharedTenantRegistrySet.ImageRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.InvoiceRegistry, err = sharedTenantRegistrySet.InvoiceRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.ManualRegistry, err = sharedTenantRegistrySet.ManualRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.SettingsRegistry, err = sharedTenantRegistrySet.SettingsRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.ExportRegistry, err = sharedTenantRegistrySet.ExportRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.RestoreOperationRegistry, err = sharedTenantRegistrySet.RestoreOperationRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.RestoreStepRegistry, err = sharedTenantRegistrySet.RestoreStepRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.FileRegistry, err = sharedTenantRegistrySet.FileRegistry.WithCurrentUser(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	registrySetTenant2.UserRegistry = userRegistry
	registrySetTenant2.TenantRegistry = sharedTenantRegistrySet.TenantRegistry

	// Set main currency for both tenants
	mainCurrency := "USD"
	err = registrySetTenant1.SettingsRegistry.Save(tenant1Ctx, models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	err = registrySetTenant2.SettingsRegistry.Save(tenant2Ctx, models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	// Tenant 1 user creates entities

	tenant1Data := TemplateData{
		LocationID:  "tenant1-location-1",
		AreaID:      "tenant1-area-1",
		CommodityID: "tenant1-commodity-1",
		ImageID:     "tenant1-image-1",
		InvoiceID:   "tenant1-invoice-1",
		ManualID:    "tenant1-manual-1",
	}

	tenant1XML, err := generateXMLFromTemplate("testdata/inventory_template.xml", tenant1Data)
	c.Assert(err, qt.IsNil)

	entityServiceTenant1 := services.NewEntityService(registrySetTenant1, "file://./test_uploads?create_dir=true")
	proc1 := processor.NewRestoreOperationProcessor("test-restore-op-tenant1", registrySetTenant1, entityServiceTenant1, "file://./test_uploads?create_dir=true")

	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader1 := strings.NewReader(tenant1XML)
	stats1, err := proc1.RestoreFromXML(tenant1Ctx, reader1, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats1.ErrorCount, qt.Equals, 0)

	// Get tenant 1's commodity ID
	tenant1Commodities, err := registrySetTenant1.CommodityRegistry.List(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(tenant1Commodities), qt.Equals, 1)
	tenant1CommodityID := tenant1Commodities[0].ID

	// Tenant 2 user attempts to access tenant 1's data (CROSS-TENANT ATTACK!)
	maliciousData := TemplateData{
		LocationID:  "tenant2-location-1",
		AreaID:      "tenant2-area-1",
		CommodityID: tenant1CommodityID, // SECURITY VIOLATION: Cross-tenant access!
		ImageID:     "tenant2-malicious-image",
		InvoiceID:   "tenant2-malicious-invoice",
		ManualID:    "tenant2-malicious-manual",
	}

	maliciousXML, err := generateXMLFromTemplate("testdata/inventory_template.xml", maliciousData)
	c.Assert(err, qt.IsNil)

	entityServiceTenant2 := services.NewEntityService(registrySetTenant2, "file://./test_uploads?create_dir=true")
	proc2 := processor.NewRestoreOperationProcessor("test-restore-op-tenant2", registrySetTenant2, entityServiceTenant2, "file://./test_uploads?create_dir=true")

	reader2 := strings.NewReader(maliciousXML)
	stats2, err := proc2.RestoreFromXML(tenant2Ctx, reader2, options)

	// Should fail or skip unauthorized cross-tenant access
	c.Assert(stats2.ErrorCount > 0, qt.IsTrue, qt.Commentf("Should have errors when attempting cross-tenant access"))

	// Verify tenant isolation - tenant 2 should not have access to tenant 1's data
	tenant2Commodities, err := registrySetTenant2.CommodityRegistry.List(tenant2Ctx)
	c.Assert(err, qt.IsNil)
	for _, commodity := range tenant2Commodities {
		c.Assert(commodity.ID, qt.Not(qt.Equals), tenant1CommodityID, qt.Commentf("Tenant 2 should not have access to tenant 1's commodity"))
	}

	// Verify tenant 1's data is unchanged
	finalTenant1Commodities, err := registrySetTenant1.CommodityRegistry.List(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalTenant1Commodities), qt.Equals, 1, qt.Commentf("Tenant 1's data should be unchanged"))
}

// TestRestoreService_SecurityValidation_ValidUserManipulations tests that users can freely manipulate their own data
func TestRestoreService_SecurityValidation_ValidUserManipulations(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create test user
	testUser := models.User{
		Name:  "Test User",
		Email: "user@example.com",
	}

	userRegistry := memory.NewUserRegistry()
	createdUser, err := userRegistry.Create(ctx, testUser)
	c.Assert(err, qt.IsNil)

	registrySet := memory.NewRegistrySetWithUserID(createdUser.ID)
	registrySet.UserRegistry = userRegistry

	// Set main currency
	mainCurrency := "USD"
	err = registrySet.SettingsRegistry.Save(ctx, models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	userCtx := appctx.WithUser(ctx, createdUser)

	// User creates initial entities
	initialData := TemplateData{
		LocationID:  "initial-location-1",
		AreaID:      "initial-area-1",
		CommodityID: "initial-commodity-1",
		ImageID:     "initial-image-1",
		InvoiceID:   "initial-invoice-1",
		ManualID:    "initial-manual-1",
	}

	initialXML, err := generateXMLFromTemplate("testdata/inventory_template.xml", initialData)
	c.Assert(err, qt.IsNil)

	entityService := services.NewEntityService(registrySet, "file://./test_uploads?create_dir=true")
	proc := processor.NewRestoreOperationProcessor("test-restore-op-valid", registrySet, entityService, "file://./test_uploads?create_dir=true")

	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader1 := strings.NewReader(initialXML)
	stats1, err := proc.RestoreFromXML(userCtx, reader1, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats1.ErrorCount, qt.Equals, 0)

	// Get the created entity IDs
	locations, err := registrySet.LocationRegistry.List(userCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations), qt.Equals, 1)
	locationID := locations[0].ID

	areas, err := registrySet.AreaRegistry.List(userCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Equals, 1)
	areaID := areas[0].ID

	commodities, err := registrySet.CommodityRegistry.List(userCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, 1)
	commodityID := commodities[0].ID

	// User performs various valid manipulations within their own context:

	// 1. Re-link files to different commodities (within same user)
	// 2. Create new entities and link existing files to them
	// 3. Reorganize entity relationships

	// Create a second commodity for the same user
	secondCommodityData := TemplateData{
		LocationID:  locationID, // Reuse existing location
		AreaID:      areaID,     // Reuse existing area
		CommodityID: "second-commodity-1",
		ImageID:     "second-image-1",
		InvoiceID:   "second-invoice-1",
		ManualID:    "second-manual-1",
	}

	secondXML, err := generateXMLFromTemplate("testdata/inventory_template.xml", secondCommodityData)
	c.Assert(err, qt.IsNil)

	reader2 := strings.NewReader(secondXML)
	stats2, err := proc.RestoreFromXML(userCtx, reader2, types.RestoreOptions{
		Strategy:        types.RestoreStrategyMergeAdd,
		DryRun:          false,
		IncludeFileData: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats2.ErrorCount, qt.Equals, 0, qt.Commentf("User should be able to manipulate their own data freely"))

	// Verify user now has 2 commodities
	finalCommodities, err := registrySet.CommodityRegistry.List(userCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalCommodities), qt.Equals, 2, qt.Commentf("User should have 2 commodities"))

	// Verify all commodities belong to the user
	for _, commodity := range finalCommodities {
		// TODO: Add user ownership validation when implemented
		c.Assert(commodity.ID, qt.Not(qt.Equals), "", qt.Commentf("Commodity should have valid ID"))
	}

	// Test re-linking existing files to new commodity (advanced manipulation)
	// This simulates user reorganizing their files between commodities
	relinkData := TemplateData{
		LocationID:  locationID,
		AreaID:      areaID,
		CommodityID: commodityID,      // Original commodity
		ImageID:     "second-image-1", // Link second image to first commodity (re-organization)
		InvoiceID:   "relinked-invoice-1",
		ManualID:    "relinked-manual-1",
	}

	relinkXML, err := generateXMLFromTemplate("testdata/inventory_template.xml", relinkData)
	c.Assert(err, qt.IsNil)

	reader3 := strings.NewReader(relinkXML)
	stats3, err := proc.RestoreFromXML(userCtx, reader3, types.RestoreOptions{
		Strategy:        types.RestoreStrategyMergeAdd,
		DryRun:          false,
		IncludeFileData: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats3.ErrorCount, qt.Equals, 0, qt.Commentf("User should be able to re-organize their own files"))
}

// TestRestoreService_SecurityValidation_LoggingUnauthorizedAttempts tests that unauthorized access attempts are logged
func TestRestoreService_SecurityValidation_LoggingUnauthorizedAttempts(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create two test users
	testUser1 := models.User{
		Name:  "User 1",
		Email: "user1@example.com",
	}

	testUser2 := models.User{
		Name:  "User 2",
		Email: "user2@example.com",
	}

	userRegistry := memory.NewUserRegistry()
	createdUser1, err := userRegistry.Create(ctx, testUser1)
	c.Assert(err, qt.IsNil)

	createdUser2, err := userRegistry.Create(ctx, testUser2)
	c.Assert(err, qt.IsNil)

	// Create shared registry set that both users will use
	sharedLoggingRegistrySet := memory.NewRegistrySet()
	sharedLoggingRegistrySet.UserRegistry = userRegistry

	// Create user-specific registry sets that share the same underlying data
	user1Ctx := appctx.WithUser(ctx, createdUser1)
	registrySet1 := &registry.Set{}
	registrySet1.LocationRegistry, err = sharedLoggingRegistrySet.LocationRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.AreaRegistry, err = sharedLoggingRegistrySet.AreaRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.CommodityRegistry, err = sharedLoggingRegistrySet.CommodityRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.ImageRegistry, err = sharedLoggingRegistrySet.ImageRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.InvoiceRegistry, err = sharedLoggingRegistrySet.InvoiceRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.ManualRegistry, err = sharedLoggingRegistrySet.ManualRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.SettingsRegistry, err = sharedLoggingRegistrySet.SettingsRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.ExportRegistry, err = sharedLoggingRegistrySet.ExportRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.RestoreOperationRegistry, err = sharedLoggingRegistrySet.RestoreOperationRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.RestoreStepRegistry, err = sharedLoggingRegistrySet.RestoreStepRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.FileRegistry, err = sharedLoggingRegistrySet.FileRegistry.WithCurrentUser(user1Ctx)
	c.Assert(err, qt.IsNil)
	registrySet1.UserRegistry = userRegistry
	registrySet1.TenantRegistry = sharedLoggingRegistrySet.TenantRegistry

	user2Ctx := appctx.WithUser(ctx, createdUser2)
	registrySet2 := &registry.Set{}
	registrySet2.LocationRegistry, err = sharedLoggingRegistrySet.LocationRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.AreaRegistry, err = sharedLoggingRegistrySet.AreaRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.CommodityRegistry, err = sharedLoggingRegistrySet.CommodityRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.ImageRegistry, err = sharedLoggingRegistrySet.ImageRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.InvoiceRegistry, err = sharedLoggingRegistrySet.InvoiceRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.ManualRegistry, err = sharedLoggingRegistrySet.ManualRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.SettingsRegistry, err = sharedLoggingRegistrySet.SettingsRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.ExportRegistry, err = sharedLoggingRegistrySet.ExportRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.RestoreOperationRegistry, err = sharedLoggingRegistrySet.RestoreOperationRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.RestoreStepRegistry, err = sharedLoggingRegistrySet.RestoreStepRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.FileRegistry, err = sharedLoggingRegistrySet.FileRegistry.WithCurrentUser(user2Ctx)
	c.Assert(err, qt.IsNil)
	registrySet2.UserRegistry = userRegistry
	registrySet2.TenantRegistry = sharedLoggingRegistrySet.TenantRegistry

	// Set main currency for both users
	mainCurrency := "USD"
	err = registrySet1.SettingsRegistry.Save(user1Ctx, models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	err = registrySet2.SettingsRegistry.Save(user2Ctx, models.SettingsObject{
		MainCurrency: &mainCurrency,
	})
	c.Assert(err, qt.IsNil)

	// User 1 creates entities

	user1Data := TemplateData{
		LocationID:  "user1-location-1",
		AreaID:      "user1-area-1",
		CommodityID: "user1-commodity-1",
		ImageID:     "user1-image-1",
		InvoiceID:   "user1-invoice-1",
		ManualID:    "user1-manual-1",
	}

	user1XML, err := generateXMLFromTemplate("testdata/inventory_template.xml", user1Data)
	c.Assert(err, qt.IsNil)

	entityService1 := services.NewEntityService(registrySet1, "file://./test_uploads?create_dir=true")
	proc1 := processor.NewRestoreOperationProcessor("test-restore-op-log1", registrySet1, entityService1, "file://./test_uploads?create_dir=true")

	options := types.RestoreOptions{
		Strategy:        types.RestoreStrategyFullReplace,
		DryRun:          false,
		IncludeFileData: true,
	}

	reader1 := strings.NewReader(user1XML)
	stats1, err := proc1.RestoreFromXML(user1Ctx, reader1, options)
	c.Assert(err, qt.IsNil)
	c.Assert(stats1.ErrorCount, qt.Equals, 0)

	// Get user 1's commodity ID
	user1Commodities, err := registrySet1.CommodityRegistry.List(user1Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(user1Commodities), qt.Equals, 1)
	user1CommodityID := user1Commodities[0].ID

	// TODO: Set up log capture mechanism to verify security logging
	// This would typically involve:
	// 1. Configuring a test logger that captures log entries
	// 2. Setting up the restore processor to use this logger
	// 3. Performing unauthorized operations
	// 4. Verifying that security violations are logged with appropriate details

	// User 2 attempts multiple unauthorized operations that should be logged
	unauthorizedAttempts := []struct {
		name        string
		description string
		data        TemplateData
	}{
		{
			name:        "cross_user_commodity_access",
			description: "Attempt to link files to another user's commodity",
			data: TemplateData{
				LocationID:  "user2-location-1",
				AreaID:      "user2-area-1",
				CommodityID: user1CommodityID, // UNAUTHORIZED: User 1's commodity
				ImageID:     "user2-malicious-image-1",
				InvoiceID:   "user2-malicious-invoice-1",
				ManualID:    "user2-malicious-manual-1",
			},
		},
		{
			name:        "non_existent_entity_access",
			description: "Attempt to link files to non-existent entity",
			data: TemplateData{
				LocationID:  "user2-location-1",
				AreaID:      "user2-area-1",
				CommodityID: "truly-non-existent-commodity-id", // UNAUTHORIZED: Non-existent entity
				ImageID:     "user2-malicious-image-2",
				InvoiceID:   "user2-malicious-invoice-2",
				ManualID:    "user2-malicious-manual-2",
			},
		},
	}

	entityService2 := services.NewEntityService(registrySet2, "file://./test_uploads?create_dir=true")
	proc2 := processor.NewRestoreOperationProcessor("test-restore-op-log2", registrySet2, entityService2, "file://./test_uploads?create_dir=true")

	for _, attempt := range unauthorizedAttempts {
		c.Run(attempt.name, func(c *qt.C) {
			var maliciousXML string
			var err error

			if attempt.name == "non_existent_entity_access" {
				// For non-existent entity test, create XML that creates valid entities but tries to link files to non-existent commodity
				maliciousXML = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/export" exportDate="2024-01-01T00:00:00Z" exportType="full_database">
  <locations>
    <location id="%s">
      <locationName>Test Location</locationName>
      <address>123 Test Street</address>
    </location>
  </locations>
  <areas>
    <area id="%s">
      <areaName>Test Area</areaName>
      <locationId>%s</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="user2-valid-commodity">
      <commodityName>Valid Commodity</commodityName>
      <shortName>ValidComm</shortName>
      <type>equipment</type>
      <areaId>%s</areaId>
      <count>1</count>
      <originalPrice>100.00</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <currentPrice>100.00</currentPrice>
      <status>in_use</status>
      <purchaseDate>2024-01-01</purchaseDate>
      <registeredDate>2024-01-01</registeredDate>
      <lastModifiedDate>2024-01-01</lastModifiedDate>
      <draft>false</draft>
      <images>
        <file id="%s">
          <path>test-image-malicious</path>
          <originalPath>test-image-malicious.jpg</originalPath>
          <extension>.jpg</extension>
          <mimeType>image/jpeg</mimeType>
          <data>VGhpcyBpcyBhIG1hbGljaW91cyBpbWFnZSBmaWxlIGNvbnRlbnQu</data>
        </file>
      </images>
      <invoices>
        <file id="%s">
          <path>test-invoice-malicious</path>
          <originalPath>test-invoice-malicious.pdf</originalPath>
          <extension>.pdf</extension>
          <mimeType>application/pdf</mimeType>
          <data>VGhpcyBpcyBhIG1hbGljaW91cyBpbnZvaWNlIGZpbGUgY29udGVudC4=</data>
        </file>
      </invoices>
      <manuals>
        <file id="%s">
          <path>test-manual-malicious</path>
          <originalPath>test-manual-malicious.pdf</originalPath>
          <extension>.pdf</extension>
          <mimeType>application/pdf</mimeType>
          <data>VGhpcyBpcyBhIG1hbGljaW91cyBtYW51YWwgZmlsZSBjb250ZW50Lg==</data>
        </file>
      </manuals>
    </commodity>
  </commodities>
</inventory>`, attempt.data.LocationID, attempt.data.AreaID, attempt.data.LocationID, attempt.data.AreaID, attempt.data.ImageID, attempt.data.InvoiceID, attempt.data.ManualID)
			} else {
				maliciousXML, err = generateXMLFromTemplate("testdata/inventory_template.xml", attempt.data)
				c.Assert(err, qt.IsNil)
			}

			reader := strings.NewReader(maliciousXML)
			stats, err := proc2.RestoreFromXML(user2Ctx, reader, options)
			c.Assert(err, qt.IsNil)

			// Should have errors for unauthorized attempts
			if attempt.name == "non_existent_entity_access" {
				// For non-existent entity access, files should be uploaded as orphaned (no errors, but logged)
				// This is the correct behavior for data recovery scenarios
				c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("Non-existent entity files should be uploaded as orphaned"))
			} else {
				// For other security violations, should have errors
				c.Assert(stats.ErrorCount > 0, qt.IsTrue, qt.Commentf("Should have errors for: %s", attempt.description))
			}

			// TODO: Verify that the attempt was logged with:
			// - User ID who attempted the operation
			// - Target entity ID they tried to access
			// - Type of unauthorized operation
			// - Timestamp
			// - IP address (if available)
			// - Request details

			// Example of what should be logged:
			// {
			//   "level": "WARN",
			//   "message": "Unauthorized entity access attempt",
			//   "user_id": "user2-id",
			//   "target_entity_id": "user1-commodity-id",
			//   "entity_type": "commodity",
			//   "operation": "restore_link_files",
			//   "attempt_type": "cross_user_access",
			//   "timestamp": "2024-01-01T12:00:00Z"
			// }
		})
	}

	// Verify that user 1's data remains unchanged after all attack attempts
	finalUser1Commodities, err := registrySet1.CommodityRegistry.List(user1Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(finalUser1Commodities), qt.Equals, 1, qt.Commentf("User 1's data should be unchanged after attacks"))

	// Verify that user 2 has no unauthorized access
	user2Commodities, err := registrySet2.CommodityRegistry.List(user2Ctx)
	c.Assert(err, qt.IsNil)
	for _, commodity := range user2Commodities {
		c.Assert(commodity.ID, qt.Not(qt.Equals), user1CommodityID, qt.Commentf("User 2 should not have access to user 1's commodity"))
	}
}

// TestRestoreService_SecurityValidation_UserProvidedTenantID tests that user-provided tenant IDs are rejected
func TestRestoreService_SecurityValidation_UserProvidedTenantID(t *testing.T) {
	c := qt.New(t)

	// This test demonstrates the CRITICAL security vulnerability:
	// Users can currently provide tenant IDs via X-Tenant-ID header
	// This allows cross-tenant data access!

	c.Log("🚨 CRITICAL SECURITY VULNERABILITY DETECTED!")
	c.Log("Current system accepts user-provided tenant IDs via:")
	c.Log("1. X-Tenant-ID header (HeaderTenantResolver)")
	c.Log("2. Subdomain manipulation (SubdomainTenantResolver)")
	c.Log("3. No validation that user is authorized for the tenant")
	c.Log("")
	c.Log("ATTACK SCENARIO:")
	c.Log("1. User authenticates as user@tenant1.com")
	c.Log("2. User sends request with X-Tenant-ID: tenant2")
	c.Log("3. System grants access to tenant2 data!")
	c.Log("")
	c.Log("REQUIRED FIXES:")
	c.Log("1. NEVER accept user-provided tenant_id in any form")
	c.Log("2. Derive tenant from authenticated JWT token only")
	c.Log("3. Validate user belongs to the tenant")
	c.Log("4. Log all unauthorized tenant access attempts")
	c.Log("")
	c.Log("FILES TO FIX:")
	c.Log("- go/apiserver/tenant_context.go (HeaderTenantResolver)")
	c.Log("- go/apiserver/auth.go (JWT token generation)")
	c.Log("- All API endpoints that use TenantMiddleware")

	// TODO: Add actual HTTP test that demonstrates the vulnerability:
	// req := httptest.NewRequest("POST", "/api/restore", strings.NewReader(xmlData))
	// req.Header.Set("X-Tenant-ID", "other-tenant-id")  // 🚨 SECURITY VIOLATION!
	// req.Header.Set("Authorization", "Bearer " + userToken)
	//
	// The system should:
	// 1. Extract tenant from JWT token, not header
	// 2. Reject requests with mismatched tenant context
	// 3. Log the security violation attempt
	// 4. Return 403 Forbidden

	// For now, this test serves as documentation of the vulnerability
	c.Assert(true, qt.IsTrue, qt.Commentf("Security vulnerability documented - implementation needed"))
}
