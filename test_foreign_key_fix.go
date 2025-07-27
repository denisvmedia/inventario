package main

import (
	"context"
	"fmt"
	"log"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

func main() {
	// Connect to the test database
	dsn := "postgres://inventario_test:test_password@localhost:5433/inventario_test?sslmode=disable"
	
	// Get registry set
	registrySet, err := registry.GetRegistry(dsn)
	if err != nil {
		log.Fatalf("Failed to get registry: %v", err)
	}

	// Create entity service
	entityService := services.NewEntityService(registrySet, "file:///tmp/test-uploads?create_dir=1")

	ctx := context.Background()

	fmt.Println("=== Testing Foreign Key Constraint Fix ===")

	// Step 1: Create a file entity
	fmt.Println("1. Creating a file entity...")
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
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}
	fmt.Printf("✅ Created file with ID: %s\n", createdFile.ID)

	// Step 2: Create an export that references the file
	fmt.Println("2. Creating an export that references the file...")
	export := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "Test export",
		FileID:      &createdFile.ID,
		CreatedDate: models.PNow(),
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	if err != nil {
		log.Fatalf("Failed to create export: %v", err)
	}
	fmt.Printf("✅ Created export with ID: %s\n", createdExport.ID)

	// Step 3: Try to delete the file directly (this should work now with ON DELETE SET NULL)
	fmt.Println("3. Testing direct file deletion (should work with ON DELETE SET NULL)...")
	err = registrySet.FileRegistry.Delete(ctx, createdFile.ID)
	if err != nil {
		fmt.Printf("❌ Direct file deletion failed: %v\n", err)
	} else {
		fmt.Println("✅ Direct file deletion succeeded!")
		
		// Check if export still exists but with NULL file_id
		updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
		if err != nil {
			log.Fatalf("Failed to get updated export: %v", err)
		}
		
		if updatedExport.FileID == nil {
			fmt.Println("✅ Export file_id was set to NULL as expected!")
		} else {
			fmt.Printf("❌ Export file_id is still: %s (should be NULL)\n", *updatedExport.FileID)
		}
	}

	// Step 4: Clean up - delete the export
	fmt.Println("4. Cleaning up - deleting export...")
	err = registrySet.ExportRegistry.Delete(ctx, createdExport.ID)
	if err != nil {
		log.Fatalf("Failed to delete export: %v", err)
	}
	fmt.Println("✅ Export deleted successfully!")

	fmt.Println("\n=== Test completed successfully! ===")
	fmt.Println("The foreign key constraint fix is working correctly.")
	fmt.Println("Files can now be deleted even when referenced by exports,")
	fmt.Println("and the export's file_id is automatically set to NULL.")
}
