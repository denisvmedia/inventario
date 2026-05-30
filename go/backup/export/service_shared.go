package export

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ExtractTenantUserFromContext extracts tenant and user IDs from context.
// Returns an error if context is missing required tenant/user information.
func ExtractTenantUserFromContext(ctx context.Context) (tenantID, userID string, err error) {
	// Try to extract user from context using proper typed keys
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return "", "", errors.New("user context is required but not found")
	}

	// Extract user ID
	userID = user.ID
	if userID == "" {
		return "", "", errors.New("user ID is empty in context")
	}

	// Extract tenant ID from user
	tenantID = user.TenantID
	if tenantID == "" {
		return "", "", errors.New("tenant ID is empty in user context")
	}

	return tenantID, userID, nil
}

// ExportArgs contains arguments for export operations
type ExportArgs struct {
	IncludeFileData bool
}

// ExportService handles the background processing of export requests.
//
// The signer field is used by the default `.inb` exporter (#534) to sign the
// generated archive. Under the legacy_xml_backup build the signer is accepted
// by the constructor but ignored — keeping the constructor signature identical
// across builds so the untagged worker/bootstrap wiring never branches.
type ExportService struct {
	factorySet     *registry.FactorySet
	uploadLocation string
	signer         *backupsign.Signer
}

// NewExportService creates a new export service. The signer is consumed by the
// default `.inb` exporter and ignored by the legacy XML exporter; the signature
// is identical across both builds on purpose.
func NewExportService(factorySet *registry.FactorySet, uploadLocation string, signer *backupsign.Signer) *ExportService {
	return &ExportService{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
		signer:         signer,
	}
}

// ProcessExport processes an export request in the background. It is
// format-agnostic: the per-build generateExport produces either a signed `.inb`
// archive (default) or a legacy XML bundle, and createExportFileEntity stamps
// the matching Ext/MIME/LinkedEntityMeta via the per-build exportFileMeta.
func (s *ExportService) ProcessExport(ctx context.Context, exportID string) error {
	// Get the export request
	export, err := s.factorySet.ExportRegistryFactory.CreateServiceRegistry().Get(ctx, exportID)
	if err != nil {
		return errxtrace.Wrap("failed to get export", err)
	}

	// Skip processing for imported exports - they are already completed
	if export.Type == models.ExportTypeImported {
		return nil
	}

	user, err := s.factorySet.UserRegistry.Get(ctx, export.CreatedByUserID)
	if err != nil {
		return errxtrace.Wrap("failed to get user", err)
	}

	// The export worker drives ProcessExport from a background context, so
	// no request-time middleware has populated user/group context. Resolve
	// the export's group now and inject both into ctx — the downstream
	// registry factories and createExportFileEntity read them from there.
	group, err := s.factorySet.LocationGroupRegistry.Get(ctx, export.GroupID)
	if err != nil {
		return errxtrace.Wrap("failed to get export group", err)
	}

	ctx = appctx.WithUser(ctx, user)
	ctx = appctx.WithGroup(ctx, group)

	// Update status to in_progress
	export.Status = models.ExportStatusInProgress
	expReg, err := s.factorySet.ExportRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get export registry", err)
	}
	_, err = expReg.Update(ctx, *export)
	if err != nil {
		return errxtrace.Wrap("failed to update export status", err)
	}

	// Generate the export and collect statistics using user context
	filePath, stats, err := s.generateExport(ctx, *export)
	if err != nil {
		// Update status to failed
		export.Status = models.ExportStatusFailed
		export.ErrorMessage = err.Error()
		_, expErr := s.factorySet.ExportRegistryFactory.CreateServiceRegistry().Update(ctx, *export)
		return errxtrace.Wrap("failed to generate export", errors.Join(err, expErr))
	}

	// Probe size before creating the FileEntity — both rows need it: the
	// export keeps a denormalized FileSize and the file row needs SizeBytes
	// for the per-group storage-usage aggregation (#1388).
	var artifactSize int64
	if size, sizeErr := s.getFileSize(ctx, filePath); sizeErr == nil {
		artifactSize = size
	}

	// Create file entity for the export using user context
	fileEntity, err := s.createExportFileEntity(ctx, export.ID, export.Description, filePath, artifactSize)
	if err != nil {
		// Update status to failed
		export.Status = models.ExportStatusFailed
		export.ErrorMessage = err.Error()
		_, updateErr := s.factorySet.ExportRegistryFactory.CreateServiceRegistry().Update(ctx, *export)
		return errxtrace.Wrap("failed to create export file entity", errors.Join(err, updateErr))
	}

	// Store statistics in export record
	export.LocationCount = stats.LocationCount
	export.AreaCount = stats.AreaCount
	export.CommodityCount = stats.CommodityCount
	export.ImageCount = stats.ImageCount
	export.InvoiceCount = stats.InvoiceCount
	export.ManualCount = stats.ManualCount
	export.FileCount = stats.FileCount
	export.BinaryDataSize = stats.BinaryDataSize
	export.FileSize = artifactSize

	// Update status to completed using user context
	export.Status = models.ExportStatusCompleted
	export.FileID = &fileEntity.ID
	export.FilePath = filePath // Keep for backward compatibility during migration
	export.CompletedDate = models.PNow()
	export.ErrorMessage = ""

	userReg, err := s.factorySet.ExportRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get export registry", err)
	}

	_, err = userReg.Update(ctx, *export)
	if err != nil {
		return errxtrace.Wrap("failed to update export completion", err)
	}

	return nil
}

// createExportFileEntity creates a file entity for an export artifact. The
// format-specific fields (Ext / MIMEType / LinkedEntityMeta / Tags) come from
// the per-build exportFileMeta so the same row-construction logic serves both
// the `.inb` and legacy XML builds.
func (s *ExportService) createExportFileEntity(ctx context.Context, exportID, description, filePath string, sizeBytes int64) (*models.FileEntity, error) {
	// Extract filename from path for title
	filename := filepath.Base(filePath)
	if ext := filepath.Ext(filename); ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Extract tenant and user from context
	tenantID, userID, err := ExtractTenantUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to extract tenant/user context", err)
	}

	// FileEntity is group-scoped (group_id NOT NULL + FK on PostgreSQL),
	// so the export's group must be on the context — exports themselves
	// are always created inside a group-scoped request.
	groupID := appctx.GroupIDFromContext(ctx)
	if groupID == "" {
		return nil, errors.New("group context is required but not found")
	}

	meta := exportFileMeta()

	// Create file entity
	now := time.Now()
	fileEntity := models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenantID,
			GroupID:         groupID,
			CreatedByUserID: userID,
		},
		Title:            fmt.Sprintf("Export: %s", description),
		Description:      fmt.Sprintf("Export file generated on %s", now.Format("2006-01-02 15:04:05")),
		Type:             models.FileTypeDocument,
		Category:         models.FileCategoryOther, // Export bundles aren't user-facing files; they live outside the four UI tiles
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
			SizeBytes:    sizeBytes,
		},
	}

	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get file registry", err)
	}

	created, err := fileReg.Create(ctx, fileEntity)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file entity", err)
	}

	return created, nil
}

// exportFileMetaFields carries the format-specific FileEntity stamping for an
// export artifact. Populated by the per-build exportFileMeta.
type exportFileMetaFields struct {
	Ext              string
	MIMEType         string
	LinkedEntityMeta string
	Tags             models.StringSlice
}

// DeleteExportFile is deprecated - export files are now managed through the
// file entity system. Kept for backward compatibility.
func (s *ExportService) DeleteExportFile(ctx context.Context, filePath string) error {
	if filePath == "" {
		return nil // Nothing to delete
	}

	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer func() {
		if closeErr := b.Close(); closeErr != nil {
			err = errxtrace.Wrap("failed to close blob bucket", closeErr)
		}
	}()

	err = b.Delete(ctx, filePath)
	if err != nil {
		return errxtrace.Wrap("failed to delete export file", err)
	}

	return nil
}

// getFileSize gets the size of a file in blob storage.
func (s *ExportService) getFileSize(ctx context.Context, filePath string) (int64, error) {
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return 0, errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	attrs, err := b.Attributes(ctx, filePath)
	if err != nil {
		return 0, errxtrace.Wrap("failed to get file attributes", err)
	}

	return attrs.Size, nil
}
