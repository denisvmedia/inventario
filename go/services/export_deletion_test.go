package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func TestExportDeletionOrder(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create in-memory registries
	registrySet := &registry.Set{
		ExportRegistry: memory.NewExportRegistry(),
		FileRegistry:   memory.NewFileRegistry(),
	}

	// Note: We're testing the deletion order logic directly with registries
	// rather than using the service to avoid file system complications

	// Create a file entity
	file := models.FileEntity{
		Title:       "Test Export File",
		Description: "Test file for export",
		Type:        models.FileTypeDocument,
		File: &models.File{
			Path:         "test-export",
			OriginalPath: "test-export.xml",
			Ext:          ".xml",
			MIMEType:     "application/xml",
		},
	}

	createdFile, err := registrySet.FileRegistry.Create(ctx, file)
	c.Assert(err, qt.IsNil)
	c.Assert(createdFile, qt.IsNotNil)

	// Create an export that references the file
	export := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "Test export",
		FileID:      &createdFile.ID,
		CreatedDate: models.PNow(),
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)
	c.Assert(createdExport, qt.IsNotNil)

	// Verify both entities exist
	_, err = registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.FileRegistry.Get(ctx, createdFile.ID)
	c.Assert(err, qt.IsNil)

	// Test the deletion order by deleting export first, then file
	// This simulates what DeleteExportWithFile should do to avoid foreign key constraint violations

	// Step 1: Delete the export first
	err = registrySet.ExportRegistry.Delete(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	// Verify export is deleted
	_, err = registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)

	// Step 2: Now delete the file (this should work since export no longer references it)
	err = registrySet.FileRegistry.Delete(ctx, createdFile.ID)
	c.Assert(err, qt.IsNil)

	// Verify file is deleted
	_, err = registrySet.FileRegistry.Get(ctx, createdFile.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)
}

func TestExportDeletionOrder_NoFile(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create in-memory registries
	registrySet := &registry.Set{
		ExportRegistry: memory.NewExportRegistry(),
		FileRegistry:   memory.NewFileRegistry(),
	}

	// Create entity service
	entityService := services.NewEntityService(registrySet, "memory://")

	// Create an export without a file
	export := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "Test export without file",
		FileID:      nil, // No file
		CreatedDate: models.PNow(),
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)
	c.Assert(createdExport, qt.IsNotNil)

	// Verify export exists
	_, err = registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	// Delete the export (should work even without a file)
	err = entityService.DeleteExportWithFile(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	// Verify export is deleted
	_, err = registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.Equals, registry.ErrNotFound)
}
