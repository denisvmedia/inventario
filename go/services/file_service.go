package services

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/mimekit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services/imageprocessor"
)

// FileService provides business logic for file operations
type FileService struct {
	factorySet     *registry.FactorySet
	uploadLocation string
	imageProcessor *imageprocessor.ImageProcessor
	thumbnailSizes map[string]int // map of size name to pixel size
}

// NewFileService creates a new file service
func NewFileService(factorySet *registry.FactorySet, uploadLocation string) *FileService {
	return &FileService{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
		imageProcessor: imageprocessor.NewDefault(),
		thumbnailSizes: map[string]int{
			"small":  150,
			"medium": 300,
		},
	}
}

// GenerateThumbnails generates thumbnails for an image file
func (s *FileService) GenerateThumbnails(ctx context.Context, file *models.FileEntity) error {
	// Only generate thumbnails for supported image types
	if !mimekit.IsImage(file.MIMEType) {
		return nil // Not an error, just skip thumbnail generation
	}

	// Only support JPEG and PNG for thumbnail generation
	if !strings.HasPrefix(file.MIMEType, "image/jpeg") && !strings.HasPrefix(file.MIMEType, "image/png") {
		return nil // Skip unsupported image formats
	}

	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	// Read the original image
	reader, err := b.NewReader(ctx, file.OriginalPath, nil)
	if err != nil {
		return errxtrace.Wrap("failed to open original image", err)
	}
	defer reader.Close()

	// Decode the image
	var img image.Image
	switch file.MIMEType {
	case "image/jpeg":
		img, err = jpeg.Decode(reader)
	case "image/png":
		img, err = png.Decode(reader)
	default:
		return nil // Skip unsupported formats
	}
	if err != nil {
		return errxtrace.Wrap("failed to decode image", err)
	}

	// Generate thumbnails for each size
	for sizeName, maxSize := range s.thumbnailSizes {
		thumbnailPath := s.getThumbnailPath(file.TenantID, file.ID, sizeName)

		// Create thumbnail
		thumbnail := s.imageProcessor.CreateThumbnail(img, maxSize)

		// Save thumbnail to storage
		writer, err := b.NewWriter(ctx, thumbnailPath, nil)
		if err != nil {
			return errxtrace.Wrap("failed to create thumbnail writer", err)
		}

		// Always encode thumbnails as JPEG for consistency and smaller file sizes
		err = jpeg.Encode(writer, thumbnail, &jpeg.Options{Quality: 90})

		writer.Close()
		if err != nil {
			return errxtrace.Wrap("failed to encode thumbnail", err)
		}
	}

	return nil
}

// getThumbnailPath generates the canonical tenant-prefixed thumbnail
// blob key for the given file. All thumbnails are saved as JPEG
// regardless of the original format. The blob key shape is owned by
// blobkeys.BuildThumbnailBlobKey; the helper is preserved here only as
// the FileService's single call site so the service is the single
// derivation point for the rest of the codebase.
func (s *FileService) getThumbnailPath(tenantID, fileID, sizeName string) string {
	return blobkeys.BuildThumbnailBlobKey(tenantID, fileID, sizeName)
}

// GetThumbnailPaths returns the canonical blob keys for every
// thumbnail derived from the given file. Post-#1793 the keys are
// tenant-prefixed; `tenantID` is read off the FileEntity by the
// caller (the file row is the source of truth for which tenant owns
// the blob).
func (s *FileService) GetThumbnailPaths(tenantID, fileID string) map[string]string {
	thumbnails := make(map[string]string)
	for sizeName := range s.thumbnailSizes {
		thumbnails[sizeName] = s.getThumbnailPath(tenantID, fileID, sizeName)
	}
	return thumbnails
}

// DeleteFileWithPhysical deletes a file entity and its associated physical file
func (s *FileService) DeleteFileWithPhysical(ctx context.Context, fileID string) error {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create file registry", err)
	}

	// Get the file entity first
	file, err := fileReg.Get(ctx, fileID)
	if err != nil {
		return errxtrace.Wrap("failed to get file entity", err)
	}

	// Delete the physical file and thumbnails if they exist
	if file.File != nil && file.File.OriginalPath != "" {
		if err := s.deletePhysicalFileAndThumbnails(ctx, file.TenantID, fileID, file.File.OriginalPath, file.File.MIMEType); err != nil {
			return errxtrace.Wrap("failed to delete physical file and thumbnails", err)
		}
	}

	// Delete the file entity from database
	err = fileReg.Delete(ctx, fileID)
	if err != nil {
		return errxtrace.Wrap("failed to delete file entity", err)
	}

	return nil
}

// DeletePhysicalFile deletes only the physical file from storage
func (s *FileService) DeletePhysicalFile(ctx context.Context, filePath string) error {
	return s.deletePhysicalFile(ctx, filePath)
}

// DeletePhysicalFilesForGroup deletes every physical blob (and its thumbnails)
// that belongs to the given (tenant, group) pair. It is intended for the
// group-purge background worker and therefore uses a service-mode file
// registry to bypass tenant/group RLS. Database rows are NOT touched here —
// that is the responsibility of the GroupPurger. Callers must ensure the
// group is in pending_deletion state before invoking this method.
//
// The method is fail-fast: the first blob that fails to delete aborts the
// entire sweep so the orchestration layer can leave the group in
// pending_deletion and retry on the next tick.
func (s *FileService) DeletePhysicalFilesForGroup(ctx context.Context, tenantID, groupID string) error {
	if tenantID == "" || groupID == "" {
		return errxtrace.Wrap("tenantID and groupID are required", registry.ErrFieldRequired)
	}

	fileReg := s.factorySet.FileRegistryFactory.CreateServiceRegistry()
	files, err := fileReg.ListByGroup(ctx, tenantID, groupID)
	if err != nil {
		return errxtrace.Wrap("failed to list files by group", err)
	}

	for _, file := range files {
		if file == nil {
			continue
		}
		if file.File == nil || file.File.OriginalPath == "" {
			continue
		}
		if err := s.deletePhysicalFileAndThumbnails(ctx, file.TenantID, file.ID, file.File.OriginalPath, file.File.MIMEType); err != nil {
			return errxtrace.Wrap(fmt.Sprintf("failed to delete physical blobs for file %s", file.ID), err)
		}
	}
	return nil
}

// deletePhysicalFileAndThumbnails deletes the physical file and all its
// thumbnails. `tenantID` is the tenant that owns the row — used to
// derive the canonical thumbnail blob keys (#1793). For legacy rows
// whose blob lives under a flat key, the original-file delete uses the
// stored OriginalPath verbatim (treated as opaque) so an unbackfilled
// row still cleans up cleanly. The thumbnail-key derivation only knows
// the new layout; an unbackfilled row's legacy thumbnails are deleted
// best-effort by the backfill itself.
func (s *FileService) deletePhysicalFileAndThumbnails(ctx context.Context, tenantID, fileID, filePath, mimeType string) error {
	// Delete the original file
	if err := s.deletePhysicalFile(ctx, filePath); err != nil {
		return errxtrace.Wrap("failed to delete original file", err)
	}

	// Delete thumbnails if it's an image file
	if mimekit.IsImage(mimeType) {
		// Walk both the canonical (tenant-prefixed) and legacy
		// (flat) thumbnail keys so a row whose original blob has
		// been backfilled to a new key still has its legacy
		// thumbnails cleaned up if they happen to linger.
		thumbnailPaths := s.GetThumbnailPaths(tenantID, fileID)
		for sizeName, thumbnailPath := range thumbnailPaths {
			// Don't fail if thumbnail doesn't exist - it might not have been generated
			_ = s.deletePhysicalFile(ctx, thumbnailPath)
			// Legacy-key cleanup. No-op for already-prefixed rows
			// because the bucket-Exists check inside
			// deletePhysicalFile short-circuits on missing.
			_ = s.deletePhysicalFile(ctx, fmt.Sprintf("thumbnails/%s_%s.jpg", fileID, sizeName))
		}
	}

	return nil
}

// deletePhysicalFile is the unified implementation for deleting physical files
func (s *FileService) deletePhysicalFile(ctx context.Context, filePath string) error {
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	// Check if file exists before trying to delete it
	exists, err := b.Exists(ctx, filePath)
	if err != nil {
		return errxtrace.Wrap("failed to check if file exists", err)
	}

	if !exists {
		// File doesn't exist, nothing to delete - this is not an error
		return nil
	}

	err = b.Delete(ctx, filePath)
	if err != nil {
		return errxtrace.Wrap("failed to delete file", err)
	}

	return nil
}

// DeleteLinkedFiles deletes all files linked to a specific entity
func (s *FileService) DeleteLinkedFiles(ctx context.Context, entityType, entityID string) error {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create file registry", err)
	}

	// Get all linked files for this entity
	files, err := fileReg.ListByLinkedEntity(ctx, entityType, entityID)
	if err != nil {
		return errxtrace.Wrap("failed to get linked files", err)
	}

	// Delete all linked files (both physical and database records)
	for _, file := range files {
		err = s.DeleteFileWithPhysical(ctx, file.ID)
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Wrap("failed to delete linked file", err)
		}
	}

	return nil
}

// ThumbnailExists checks if a thumbnail file exists for a given file and size.
// `tenantID` selects the canonical tenant-prefixed namespace (#1793);
// for legacy rows whose thumbnails still live under the flat
// `thumbnails/<id>_<size>.jpg` shape, the check falls through to the
// legacy key so an unbackfilled row still surfaces an existing
// thumbnail instead of hitting placeholder generation in a loop.
func (s *FileService) ThumbnailExists(ctx context.Context, tenantID, fileID, size string) (bool, error) {
	// Validate size parameter
	if size != "small" && size != "medium" {
		return false, errxtrace.Classify(ErrInvalidThumbnailSize, errx.Attrs("size", size))
	}

	// Open bucket and check if file exists
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return false, errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	// Canonical (tenant-prefixed) location first.
	thumbnailPath := s.getThumbnailPath(tenantID, fileID, size)
	exists, err := b.Exists(ctx, thumbnailPath)
	if err != nil {
		return false, errxtrace.Wrap("failed to check thumbnail existence", err, errx.Attrs("thumbnail_path", thumbnailPath))
	}
	if exists {
		return true, nil
	}

	// Legacy flat-key fallback for rows the backfill hasn't reached.
	legacyPath := fmt.Sprintf("thumbnails/%s_%s.jpg", fileID, size)
	legacyExists, err := b.Exists(ctx, legacyPath)
	if err != nil {
		return false, errxtrace.Wrap("failed to check legacy thumbnail existence", err, errx.Attrs("thumbnail_path", legacyPath))
	}
	return legacyExists, nil
}

// ThumbnailReadPath returns the bucket-relative key from which a
// thumbnail should be read for the given file. Mirrors the resolution
// logic in ThumbnailExists: canonical tenant-prefixed key first, then
// the legacy flat key, then the canonical key as a final fallback
// (so callers calling this without checking Exists still get a stable
// answer to feed into bucket.NewReader, even if the read itself will
// fail).
func (s *FileService) ThumbnailReadPath(ctx context.Context, tenantID, fileID, size string) (string, error) {
	if size != "small" && size != "medium" {
		return "", errxtrace.Classify(ErrInvalidThumbnailSize, errx.Attrs("size", size))
	}
	canonical := s.getThumbnailPath(tenantID, fileID, size)

	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return "", errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	exists, err := b.Exists(ctx, canonical)
	if err != nil {
		return "", errxtrace.Wrap("failed to check thumbnail existence", err, errx.Attrs("thumbnail_path", canonical))
	}
	if exists {
		return canonical, nil
	}
	legacy := fmt.Sprintf("thumbnails/%s_%s.jpg", fileID, size)
	legacyExists, err := b.Exists(ctx, legacy)
	if err != nil {
		return "", errxtrace.Wrap("failed to check legacy thumbnail existence", err, errx.Attrs("thumbnail_path", legacy))
	}
	if legacyExists {
		return legacy, nil
	}
	// Neither exists — return the canonical key so the caller can fail
	// loudly against the post-migration layout.
	return canonical, nil
}
