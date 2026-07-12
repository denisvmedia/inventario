package services

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log/slog"
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

// DeleteFileWithPhysical deletes a file entity and its associated physical
// file. The ordering is row-first, blob-best-effort (#2117):
//
//  1. Resolve the file row.
//  2. Break the thumbnail_generation_jobs.file_id -> files(id) FK (NO ACTION)
//     by first deleting the user_concurrency_slots that reference each job
//     (slots.job_id -> thumbnail_generation_jobs(id), NO ACTION) and then the
//     jobs themselves.
//  3. Delete the file row.
//  4. Only after the row delete commits, best-effort delete the physical blob
//     and its thumbnails. Blob failures are swallowed/logged and never surface
//     as an error, so a missing or unreachable blob can never leave the row
//     undeleted (the previous blob-first ordering aborted before the row was
//     removed, which is what broke the restore path in processor_shared.go).
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

	// Break the thumbnail-generation chain before the file row is removed so
	// the NO ACTION FKs (slots -> jobs -> files) don't block the delete.
	if err := s.deleteThumbnailGenerationChain(ctx, fileID); err != nil {
		return errxtrace.Wrap("failed to delete thumbnail generation chain", err)
	}

	// Delete the file entity from database
	if err := fileReg.Delete(ctx, fileID); err != nil {
		return errxtrace.Wrap("failed to delete file entity", err)
	}

	// The row delete has committed. From here on, blob cleanup is best-effort:
	// a failure to remove the physical blob or its thumbnails must never undo
	// the row delete or surface as an error to the caller.
	if file.File != nil && file.File.OriginalPath != "" {
		if err := s.deletePhysicalFileAndThumbnails(ctx, file.TenantID, fileID, file.File.OriginalPath, file.File.MIMEType); err != nil {
			slog.WarnContext(ctx, "failed to delete physical blob after file row delete (best-effort)",
				"file_id", fileID, "tenant_id", file.TenantID, "error", err.Error())
		}
	}

	return nil
}

// deleteThumbnailGenerationChain removes the thumbnail-generation jobs that
// reference the given file along with the concurrency slots that reference
// those jobs, in FK-safe order (slots -> jobs). A file can own more than one
// job (a failed job plus a retry), so every job's slots must be cleared — not
// just one — or the leftover jobs' slots dangle and FK-fail the file delete on
// postgres. All registry deletes here are idempotent: a second call (or a file
// that never had a job) matches zero rows and returns nil. Each job's slots are
// cleared first because the slots.job_id -> jobs(id) FK is NO ACTION and must
// be gone before the job row can be removed.
//
// The teardown runs through SERVICE (RLS-bypassing) registries, and that is
// load-bearing: a thumbnail job is owned by the user who REQUESTED generation,
// not by the file's creator (ThumbnailGenerationService.RequestThumbnailGeneration
// stamps UserID = the requesting user on purpose, so one member cannot spend
// another's rate limit), and merely VIEWING a not-yet-generated thumbnail
// enqueues one (servePlaceholderThumbnail). The jobs table's RLS policy is
// `tenant_id = ... AND user_id = get_current_user_id()`, so a user-scoped
// teardown cannot SEE — let alone delete — a job another group member created
// for the same file. The job row then survives and the file delete trips
// fk_thumbnail_job_file (NO ACTION; PostgreSQL's RI check bypasses row
// security, so an invisible child still blocks the parent).
//
// Caller-side authorization is unaffected: DeleteFileWithPhysical has already
// resolved the file through a USER registry, so by the time the chain is torn
// down the caller has been proven to own it. Widening only the job/slot
// teardown deletes strictly the rows that reference THIS file id.
func (s *FileService) deleteThumbnailGenerationChain(ctx context.Context, fileID string) error {
	jobReg := s.factorySet.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry()

	// Resolve every job for this file so each job's slots can be cleared before
	// the job rows are deleted. No job is not an error — the file may never
	// have had thumbnails generated.
	jobs, err := jobReg.ListByFileID(ctx, fileID)
	if err != nil {
		return errxtrace.Wrap("failed to list thumbnail jobs for file", err, errx.Attrs("file_id", fileID))
	}

	if len(jobs) > 0 {
		slotReg := s.factorySet.UserConcurrencySlotRegistryFactory.CreateServiceRegistry()
		for _, job := range jobs {
			if slotErr := slotReg.DeleteByJobID(ctx, job.ID); slotErr != nil {
				return errxtrace.Wrap("failed to delete concurrency slots for thumbnail job", slotErr, errx.Attrs("file_id", fileID, "job_id", job.ID))
			}
		}
	}

	// Delete every job referencing the file (idempotent; no-op on zero rows).
	if err := jobReg.DeleteByFileID(ctx, fileID); err != nil {
		return errxtrace.Wrap("failed to delete thumbnail generation jobs for file", err, errx.Attrs("file_id", fileID))
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

// DeletePhysicalFilesForTenant deletes every physical blob (and its
// thumbnails) that belongs to the given tenant. It is the tenant-level
// analogue of DeletePhysicalFilesForGroup, intended for the admin tenant
// hard-delete flow (#2115), and likewise uses a service-mode file registry to
// bypass tenant/group RLS. Database rows are NOT touched here — that is the
// responsibility of the TenantPurger.
//
// Unlike the group variant there is no ListByGroup-style narrowing index for a
// whole tenant, so this lists every file in service mode and filters by
// TenantID in application code (the same O(total_files) scan the group purger
// avoids, but acceptable for a one-shot administrative hard-delete).
//
// The method is fail-fast: the first blob that fails to delete aborts the
// entire sweep so the orchestration layer can leave the tenant intact and the
// operator can retry once object storage is healthy.
func (s *FileService) DeletePhysicalFilesForTenant(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return errxtrace.Wrap("tenantID is required", registry.ErrFieldRequired)
	}

	fileReg := s.factorySet.FileRegistryFactory.CreateServiceRegistry()
	files, err := fileReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list files for tenant", err)
	}

	for _, file := range files {
		if file == nil || file.TenantID != tenantID {
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

// ResolveThumbnail resolves the bucket-relative key from which a
// thumbnail should be read for the given file, and whether it actually
// exists in the bucket. Canonical tenant-prefixed key (#1793) is tried
// first; legacy flat-key fallback (`thumbnails/<id>_<size>.jpg`) lets
// rows that pre-date the backfill keep rendering. When neither key
// exists, the canonical key is returned with `exists=false` so callers
// can render a placeholder against the post-migration layout.
//
// One bucket open per call — combines what `ThumbnailExists` and
// `ThumbnailReadPath` did in separate passes.
func (s *FileService) ResolveThumbnail(ctx context.Context, tenantID, fileID, size string) (path string, exists bool, err error) {
	if size != "small" && size != "medium" {
		return "", false, errxtrace.Classify(ErrInvalidThumbnailSize, errx.Attrs("size", size))
	}

	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return "", false, errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	canonical := s.getThumbnailPath(tenantID, fileID, size)
	canonicalExists, err := b.Exists(ctx, canonical)
	if err != nil {
		return "", false, errxtrace.Wrap("failed to check thumbnail existence", err, errx.Attrs("thumbnail_path", canonical))
	}
	if canonicalExists {
		return canonical, true, nil
	}

	legacy := fmt.Sprintf("thumbnails/%s_%s.jpg", fileID, size)
	legacyExists, err := b.Exists(ctx, legacy)
	if err != nil {
		return "", false, errxtrace.Wrap("failed to check legacy thumbnail existence", err, errx.Attrs("thumbnail_path", legacy))
	}
	if legacyExists {
		return legacy, true, nil
	}
	return canonical, false, nil
}
