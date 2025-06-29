package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// migrateExportsToFileEntities migrates existing exports to use file entities
func migrateExportsToFileEntities(ctx context.Context, registrySet *registry.Set) error {
	// Get all exports that have file_path but no file_id
	exports, err := registrySet.ExportRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list exports")
	}

	migrated := 0
	for _, export := range exports {
		// Skip exports that already have file_id or don't have file_path
		if export.FileID != "" || export.FilePath == "" {
			continue
		}

		// Create file entity for this export
		fileEntity, err := createFileEntityFromExport(ctx, registrySet, export)
		if err != nil {
			log.Printf("Failed to create file entity for export %s: %v", export.ID, err)
			continue
		}

		// Update export with file_id
		export.FileID = fileEntity.ID
		_, err = registrySet.ExportRegistry.Update(ctx, *export)
		if err != nil {
			log.Printf("Failed to update export %s with file_id: %v", export.ID, err)
			continue
		}

		migrated++
		log.Printf("Migrated export %s to use file entity %s", export.ID, fileEntity.ID)
	}

	log.Printf("Successfully migrated %d exports to use file entities", migrated)
	return nil
}

// createFileEntityFromExport creates a file entity from an existing export
func createFileEntityFromExport(ctx context.Context, registrySet *registry.Set, export *models.Export) (*models.FileEntity, error) {
	// Extract filename from path for title
	filename := filepath.Base(export.FilePath)
	if ext := filepath.Ext(filename); ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Create file entity
	now := time.Now()
	fileEntity := models.FileEntity{
		Title:            fmt.Sprintf("Export: %s", export.Description),
		Description:      fmt.Sprintf("Export file generated on %s", export.CreatedDate.ToTime().Format("2006-01-02 15:04:05")),
		Type:             models.FileTypeDocument, // XML files are documents
		Tags:             []string{"export", "xml"},
		LinkedEntityType: "export",
		LinkedEntityID:   export.ID,
		LinkedEntityMeta: "xml-1.0", // Mark as export file with version
		CreatedAt:        export.CreatedDate.ToTime(),
		UpdatedAt:        now,
		File: &models.File{
			Path:         filename,
			OriginalPath: export.FilePath,
			Ext:          ".xml",
			MIMEType:     "application/xml",
		},
	}

	// Create the file entity
	created, err := registrySet.FileRegistry.Create(ctx, fileEntity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create file entity")
	}

	return created, nil
}

func main() {
	var (
		dbURL = flag.String("db", "", "Database URL (required)")
		dryRun = flag.Bool("dry-run", false, "Show what would be migrated without making changes")
	)
	flag.Parse()

	if *dbURL == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -db <database-url>\n", os.Args[0])
		os.Exit(1)
	}

	if *dryRun {
		log.Println("DRY RUN MODE - no changes will be made")
	}

	ctx := context.Background()

	// Initialize registry
	registrySetFunc, cleanup := postgres.NewRegistrySet()
	defer cleanup()

	registrySet, err := registrySetFunc(registry.Config(*dbURL))
	if err != nil {
		log.Fatalf("Failed to initialize registry: %v", err)
	}

	if *dryRun {
		// In dry run mode, just count what would be migrated
		exports, err := registrySet.ExportRegistry.List(ctx)
		if err != nil {
			log.Fatalf("Failed to list exports: %v", err)
		}

		count := 0
		for _, export := range exports {
			if export.FileID == "" && export.FilePath != "" {
				count++
				log.Printf("Would migrate export %s (file: %s)", export.ID, export.FilePath)
			}
		}

		log.Printf("DRY RUN: Would migrate %d exports", count)
		return
	}

	// Perform the actual migration
	if err := migrateExportsToFileEntities(ctx, registrySet); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migration completed successfully")
}
