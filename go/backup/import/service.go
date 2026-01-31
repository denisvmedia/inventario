package importpkg

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/export/parser"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ImportService handles XML import operations for creating export records from external files
type ImportService struct {
	factorySet     *registry.FactorySet
	uploadLocation string
}

// NewImportService creates a new import service
func NewImportService(factorySet *registry.FactorySet, uploadLocation string) *ImportService {
	return &ImportService{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
	}
}

// ProcessImport processes an XML import file and updates the export record with metadata.
// It does not restore the data to the database, only extracts and sets metadata for the export record,
// and then saves it in the database.
// The export record can be later used for restore operations.
func (s *ImportService) ProcessImport(ctx context.Context, exportID, sourceFilePath string) error {
	expReg := s.factorySet.ExportRegistryFactory.CreateServiceRegistry()
	// Get the export record
	exportRecord, err := expReg.Get(ctx, exportID)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to get export record: %v", err))
	}

	// Update status to in progress
	exportRecord.Status = models.ExportStatusInProgress
	_, err = expReg.Update(ctx, *exportRecord)
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

	// Parse XML to extract metadata and statistics (without creating a new record)
	stats, _, err := parser.ParseXMLMetadata(ctx, reader)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to parse XML metadata: %v", err))
	}

	// Create file entity for the imported export
	fileEntity, err := s.createImportFileEntity(ctx, exportRecord, sourceFilePath, attrs.Size)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to create file entity: %v", err))
	}

	// Update the original export record with the parsed data
	exportRecord.Status = models.ExportStatusCompleted
	exportRecord.CompletedDate = models.PNow()
	exportRecord.FileID = &fileEntity.ID
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

	_, err = expReg.Update(ctx, *exportRecord)
	if err != nil {
		return errors.New("import was successful, but failed to update export record")
	}

	return nil
}

// createImportFileEntity creates a file entity for an imported export file
func (s *ImportService) createImportFileEntity(ctx context.Context, export *models.Export, filePath string, fileSize int64) (*models.FileEntity, error) {
	description := export.Description
	exportID := export.ID

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
	user, err := s.factorySet.UserRegistry.Get(ctx, export.UserID)
	if err != nil {
		return nil, stacktrace.Wrap("failed to get user", err)
	}
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(appctx.WithUser(ctx, user))
	if err != nil {
		return nil, stacktrace.Wrap("failed to create file registry", err)
	}
	created, err := fileReg.Create(ctx, fileEntity)
	if err != nil {
		return nil, stacktrace.Wrap("failed to create file entity", err)
	}

	return created, nil
}

// markImportFailed marks an import operation as failed with an error message
func (s *ImportService) markImportFailed(ctx context.Context, exportID, errorMessage string) error {
	expReg := s.factorySet.ExportRegistryFactory.CreateServiceRegistry()

	exportRecord, err := expReg.Get(ctx, exportID)
	if err != nil {
		return err
	}

	exportRecord.Status = models.ExportStatusFailed
	exportRecord.CompletedDate = models.PNow()
	exportRecord.ErrorMessage = errorMessage

	_, err = expReg.Update(ctx, *exportRecord)
	if err != nil {
		return err
	}

	return fmt.Errorf("%s", errorMessage)
}
