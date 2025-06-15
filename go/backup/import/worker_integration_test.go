package importpkg

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/memblob"

	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func newTestRegistrySet() *registry.Set {
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	restoreStepRegistry := memory.NewRestoreStepRegistry()

	return &registry.Set{
		LocationRegistry:         locationRegistry,
		AreaRegistry:             areaRegistry,
		CommodityRegistry:        commodityRegistry,
		ImageRegistry:            memory.NewImageRegistry(commodityRegistry),
		InvoiceRegistry:          memory.NewInvoiceRegistry(commodityRegistry),
		ManualRegistry:           memory.NewManualRegistry(commodityRegistry),
		SettingsRegistry:         memory.NewSettingsRegistry(),
		ExportRegistry:           memory.NewExportRegistry(),
		RestoreOperationRegistry: memory.NewRestoreOperationRegistry(restoreStepRegistry),
		RestoreStepRegistry:      restoreStepRegistry,
	}
}

func TestImportWorkerHandlesProcessingErrors(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"

	importService := NewImportService(registrySet, uploadLocation)

	// Setting max concurrent imports to enforce synchronous (serial) processing
	worker := NewImportWorker(importService, registrySet, 1)

	ctx := context.Background()

	// Create an import with a non-existent file path
	importExport := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Import with missing file",
		FilePath:    "non-existent-file.xml",
		Imported:    true,
	}

	createdImport, err := registrySet.ExportRegistry.Create(ctx, importExport)
	c.Assert(err, qt.IsNil)

	// Manually process the import
	worker.processPendingImports(ctx)
	// Wait for the semaphore to be released
	err = worker.semaphore.Acquire(ctx, 1)
	c.Assert(err, qt.IsNil)
	worker.semaphore.Release(1)

	// Verify that the import was processed and marked as failed
	updatedImport, err := registrySet.ExportRegistry.Get(ctx, createdImport.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedImport.Status, qt.Equals, models.ExportStatusFailed)
	// The error could be either blob bucket error or file not found error
	c.Assert(updatedImport.ErrorMessage, qt.Not(qt.Equals), "")
}

func TestImportWorkerProcessPendingImports(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for uploads
	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"

	importService := NewImportService(registrySet, uploadLocation)
	// Setting max concurrent imports to enforce synchronous (serial) processing
	worker := NewImportWorker(importService, registrySet, 1)

	ctx := context.Background()

	// Create blob bucket and upload valid XML
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)

	validXML := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/schema" exportDate="2024-01-01T00:00:00Z" exportType="commodities">
	<commodities>
		<commodity id="test-commodity-1">
			<name>Test Commodity</name>
			<type>electronics</type>
			<status>active</status>
			<count>1</count>
		</commodity>
	</commodities>
</inventory>`
	filePath := "test-import.xml"
	err = b.WriteAll(ctx, filePath, []byte(validXML), nil)
	c.Assert(err, qt.IsNil)
	b.Close()

	// Create some test imports (exports of type "imported")
	import1 := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Test import 1",
		FilePath:    filePath,
		Imported:    true,
	}

	import2 := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Test import 2",
		FilePath:    filePath,
		Imported:    true,
	}

	// Create a non-import export (should be ignored)
	regularExport := models.Export{
		Type:        models.ExportTypeCommodities,
		Status:      models.ExportStatusPending,
		Description: "Regular export",
		Imported:    false,
	}

	// Create imported export that is already completed
	completedImport := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusCompleted,
		Description: "Completed import",
		FilePath:    filePath,
		Imported:    true,
	}

	// Create imports in database
	createdImport1, err := registrySet.ExportRegistry.Create(ctx, import1)
	c.Assert(err, qt.IsNil)

	createdImport2, err := registrySet.ExportRegistry.Create(ctx, import2)
	c.Assert(err, qt.IsNil)

	createdRegularExport, err := registrySet.ExportRegistry.Create(ctx, regularExport)
	c.Assert(err, qt.IsNil)

	createdCompletedImport, err := registrySet.ExportRegistry.Create(ctx, completedImport)
	c.Assert(err, qt.IsNil)

	// Process pending imports
	worker.processPendingImports(ctx)
	// Wait for the semaphore to be released
	err = worker.semaphore.Acquire(ctx, 1)
	c.Assert(err, qt.IsNil)
	worker.semaphore.Release(1)

	// Check that imports were processed (status should change from pending)
	updatedImport1, err := registrySet.ExportRegistry.Get(ctx, createdImport1.ID)
	c.Assert(err, qt.IsNil)

	updatedImport2, err := registrySet.ExportRegistry.Get(ctx, createdImport2.ID)
	c.Assert(err, qt.IsNil)

	// Regular export should remain unchanged
	updatedRegularExport, err := registrySet.ExportRegistry.Get(ctx, createdRegularExport.ID)
	c.Assert(err, qt.IsNil)

	// Completed import should remain unchanged
	updatedCompletedImport, err := registrySet.ExportRegistry.Get(ctx, createdCompletedImport.ID)
	c.Assert(err, qt.IsNil)

	c.Assert(updatedImport1.Status, qt.Not(qt.Equals), models.ExportStatusPending, qt.Commentf("Import1 status should have changed from pending"))
	c.Assert(updatedImport2.Status, qt.Not(qt.Equals), models.ExportStatusPending, qt.Commentf("Import2 status should have changed from pending"))
	c.Assert(updatedRegularExport.Status, qt.Equals, models.ExportStatusPending, qt.Commentf("Regular export should remain pending"))
	c.Assert(updatedCompletedImport.Status, qt.Equals, models.ExportStatusCompleted, qt.Commentf("Completed import should remain completed"))
}
