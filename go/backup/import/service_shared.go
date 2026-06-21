package importpkg

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ImportService handles import operations for creating export records from
// uploaded backup files.
//
// The signer is consumed by the default `.inb` import path (which verifies the
// archive signature and reads its manifest); the legacy XML path ignores it.
// The constructor signature is identical across both builds.
type ImportService struct {
	factorySet     *registry.FactorySet
	uploadLocation string
	signer         *backupsign.Signer
}

// NewImportService creates a new import service.
func NewImportService(factorySet *registry.FactorySet, uploadLocation string, signer *backupsign.Signer) *ImportService {
	return &ImportService{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
		signer:         signer,
	}
}

// importStats is the format-agnostic statistics summary the per-build metadata
// parser returns. It is a subset of the export-side stats sufficient to stamp
// the import's export record.
type importStats struct {
	LocationCount  int
	AreaCount      int
	CommodityCount int
	ImageCount     int
	InvoiceCount   int
	ManualCount    int
	FileCount      int
	BinaryDataSize int64
}

// ProcessImport processes an uploaded backup file and updates the export record
// with metadata. It does not restore data — only extracts metadata so the
// record can later drive a restore. The per-build parseImportMetadata reads
// either a signed `.inb` manifest (default) or legacy XML.
func (s *ImportService) ProcessImport(ctx context.Context, exportID, sourceFilePath string) error {
	expReg := s.factorySet.ExportRegistryFactory.CreateServiceRegistry()
	exportRecord, err := expReg.Get(ctx, exportID)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to get export record: %v", err))
	}

	exportRecord.Status = models.ExportStatusInProgress
	if _, err = expReg.Update(ctx, *exportRecord); err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to update export status: %v", err))
	}

	// Defense-in-depth (the handler is the primary guard): the worker reads the
	// source blob key WITHOUT RLS, so refuse a path that lives outside the
	// export's own tenant namespace before opening it. An empty tenant yields an
	// empty prefix — guard it explicitly so HasPrefix can't pass vacuously.
	prefix := blobkeys.TenantPrefix(exportRecord.TenantID)
	if prefix == "" || !strings.HasPrefix(sourceFilePath, prefix) {
		return s.markImportFailed(ctx, exportID, "import source path is outside the export's tenant namespace")
	}

	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to open blob bucket: %v", err))
	}
	defer b.Close()

	reader, err := b.NewReader(ctx, sourceFilePath, nil)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to open uploaded backup file: %v", err))
	}

	attrs, err := b.Attributes(ctx, sourceFilePath)
	if err != nil {
		_ = reader.Close()
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to get file attributes: %v", err))
	}

	stats, parseErr := s.parseImportMetadata(ctx, reader)
	_ = reader.Close()
	if parseErr != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to parse backup metadata: %v", parseErr))
	}

	fileEntity, err := s.createImportFileEntity(ctx, exportRecord, sourceFilePath, attrs.Size)
	if err != nil {
		return s.markImportFailed(ctx, exportID, fmt.Sprintf("failed to create file entity: %v", err))
	}

	exportRecord.Status = models.ExportStatusCompleted
	exportRecord.CompletedDate = models.PNow()
	exportRecord.FileID = &fileEntity.ID
	exportRecord.FilePath = sourceFilePath
	exportRecord.FileSize = attrs.Size
	exportRecord.LocationCount = stats.LocationCount
	exportRecord.AreaCount = stats.AreaCount
	exportRecord.CommodityCount = stats.CommodityCount
	exportRecord.ImageCount = stats.ImageCount
	exportRecord.InvoiceCount = stats.InvoiceCount
	exportRecord.ManualCount = stats.ManualCount
	exportRecord.FileCount = stats.FileCount
	exportRecord.BinaryDataSize = stats.BinaryDataSize
	exportRecord.IncludeFileData = stats.BinaryDataSize > 0

	if _, err = expReg.Update(ctx, *exportRecord); err != nil {
		return fmt.Errorf("import was successful, but failed to update export record")
	}

	return nil
}

// createImportFileEntity creates the FileEntity backing an imported backup. The
// format-specific stamping (Ext / MIME / LinkedEntityMeta / Tags) comes from
// the per-build importFileMeta.
func (s *ImportService) createImportFileEntity(ctx context.Context, export *models.Export, filePath string, fileSize int64) (*models.FileEntity, error) {
	description := export.Description
	exportID := export.ID

	filename := filepath.Base(filePath)
	if ext := filepath.Ext(filename); ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	meta := importFileMeta()

	now := time.Now()
	fileEntity := models.FileEntity{
		Title:            fmt.Sprintf("Import: %s", description),
		Description:      fmt.Sprintf("Imported backup file uploaded on %s", now.Format("2006-01-02 15:04:05")),
		Type:             models.FileTypeDocument,
		Category:         models.FileCategoryOther,
		Tags:             meta.Tags,
		LinkedEntityType: "export",
		LinkedEntityID:   exportID,
		LinkedEntityMeta: meta.LinkedEntityMeta,
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         filename,
			OriginalPath: filePath,
			Ext:          meta.Ext,
			MIMEType:     meta.MIMEType,
			SizeBytes:    fileSize,
		},
	}

	user, err := s.factorySet.UserRegistry.Get(ctx, export.CreatedByUserID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user", err)
	}

	// The import worker runs from a background context with no request-time
	// middleware, so user/group are unset. FileEntity is group-scoped — the
	// postgres registry rejects a create without a group_id in context
	// ("group ID is required"). Resolve the export's group and inject both
	// user + group, mirroring the export worker (createExportFileEntity).
	group, err := s.factorySet.LocationGroupRegistry.Get(ctx, export.GroupID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get import group", err)
	}
	ctx = appctx.WithUser(ctx, user)
	ctx = appctx.WithGroup(ctx, group)

	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file registry", err)
	}
	created, err := fileReg.Create(ctx, fileEntity)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file entity", err)
	}

	return created, nil
}

// importFileMetaFields carries the format-specific FileEntity stamping for an
// imported backup artifact.
type importFileMetaFields struct {
	Ext              string
	MIMEType         string
	LinkedEntityMeta string
	Tags             models.StringSlice
}

// markImportFailed marks an import operation as failed.
func (s *ImportService) markImportFailed(ctx context.Context, exportID, errorMessage string) error {
	expReg := s.factorySet.ExportRegistryFactory.CreateServiceRegistry()

	exportRecord, err := expReg.Get(ctx, exportID)
	if err != nil {
		return err
	}

	exportRecord.Status = models.ExportStatusFailed
	exportRecord.CompletedDate = models.PNow()
	exportRecord.ErrorMessage = errorMessage

	if _, err = expReg.Update(ctx, *exportRecord); err != nil {
		return err
	}

	return fmt.Errorf("%s", errorMessage)
}
