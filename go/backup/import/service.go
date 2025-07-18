package importpkg

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/backup/export"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ImportService handles XML import operations for creating export records from external files
type ImportService struct {
	registrySet    *registry.Set
	uploadLocation string
}

// NewImportService creates a new import service
func NewImportService(registrySet *registry.Set, uploadLocation string) *ImportService {
	return &ImportService{
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
	}
}

// ProcessImport processes an XML import file and updates the export record with metadata.
// It does not restore the data to the database, only extracts and sets metadata for the export record,
// and then saves it in the database.
// The export record can be later used for restore operations.
func (s *ImportService) ProcessImport(ctx context.Context, exportID, sourceFilePath string) error {
	// Get the export record
	exportRecord, err := s.registrySet.ExportRegistry.Get(ctx, exportID)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to get export record: %v", err))
	}

	// Update status to in progress
	exportRecord.Status = models.ExportStatusInProgress
	_, err = s.registrySet.ExportRegistry.Update(ctx, *exportRecord)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to update export status: %v", err))
	}

	// Open blob bucket to read the XML file
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to open blob bucket: %v", err))
	}
	defer b.Close()

	// Open the uploaded XML file
	reader, err := b.NewReader(ctx, sourceFilePath, nil)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to open uploaded XML file: %v", err))
	}
	defer reader.Close()

	// Get file size
	attrs, err := b.Attributes(ctx, sourceFilePath)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to get file attributes: %v", err))
	}

	// Create export service for XML parsing
	exportService := export.NewExportService(s.registrySet, s.uploadLocation)

	// Parse XML to extract metadata and statistics (without creating a new record)
	stats, _, err := exportService.ParseXMLMetadata(ctx, reader)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to parse XML metadata: %v", err))
	}

	// Create file entity for the imported export
	fileEntity, err := s.createImportFileEntity(ctx, exportRecord.ID, exportRecord.Description, sourceFilePath, attrs.Size)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to create file entity: %v", err))
	}

	// Update the original export record with the parsed data
	exportRecord.Status = models.ExportStatusCompleted
	exportRecord.CompletedDate = models.PNow()
	exportRecord.FileID = fileEntity.ID
	exportRecord.FilePath = sourceFilePath // Keep for backward compatibility
	exportRecord.FileSize = attrs.Size
	exportRecord.LocationCount = stats.LocationCount
	exportRecord.AreaCount = stats.AreaCount
	exportRecord.CommodityCount = stats.CommodityCount
	exportRecord.ImageCount = stats.ImageCount
	exportRecord.InvoiceCount = stats.InvoiceCount
	exportRecord.ManualCount = stats.ManualCount
	exportRecord.BinaryDataSize = stats.BinaryDataSize
	exportRecord.IncludeFileData = stats.BinaryDataSize > 0

	_, err = s.registrySet.ExportRegistry.Update(ctx, *exportRecord)
	if err != nil {
		return errors.New("import was successful, but failed to update export record")
	}

	return nil
}

// createImportFileEntity creates a file entity for an imported export file
func (s *ImportService) createImportFileEntity(ctx context.Context, exportID, description, filePath string, fileSize int64) (*models.FileEntity, error) {
	// Extract filename from path for title
	filename := filepath.Base(filePath)
	if ext := filepath.Ext(filename); ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Create file entity
	now := time.Now()
	fileEntity := models.FileEntity{
		Title:            fmt.Sprintf("Import: %s", description),
		Description:      fmt.Sprintf("Imported export file uploaded on %s", now.Format("2006-01-02 15:04:05")),
		Type:             models.FileTypeDocument, // XML files are documents
		Tags:             []string{"export", "xml", "imported"},
		LinkedEntityType: "export",
		LinkedEntityID:   exportID,
		LinkedEntityMeta: "xml-1.0", // Mark as export file with version
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         filename,
			OriginalPath: filePath,
			Ext:          ".xml",
			MIMEType:     "application/xml",
		},
	}

	// Create the file entity
	created, err := s.registrySet.FileRegistry.Create(ctx, fileEntity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create file entity")
	}

	return created, nil
}

// markImportFailed marks an import operation as failed with an error message
func (s *ImportService) markImportFailed(ctx context.Context, exportID, errorMessage string) error {
	exportRecord, err := s.registrySet.ExportRegistry.Get(ctx, exportID)
	if err != nil {
		return err
	}

	exportRecord.Status = models.ExportStatusFailed
	exportRecord.CompletedDate = models.PNow()
	exportRecord.ErrorMessage = errorMessage

	_, err = s.registrySet.ExportRegistry.Update(ctx, *exportRecord)
	if err != nil {
		return err
	}

	return fmt.Errorf("%s", errorMessage)
}
