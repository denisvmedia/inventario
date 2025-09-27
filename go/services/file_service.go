package services

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
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
		return errkit.Wrap(err, "failed to open bucket")
	}
	defer b.Close()

	// Read the original image
	reader, err := b.NewReader(ctx, file.OriginalPath, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to open original image")
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
		return errkit.Wrap(err, "failed to decode image")
	}

	// Generate thumbnails for each size
	for sizeName, maxSize := range s.thumbnailSizes {
		thumbnailPath := s.getThumbnailPath(file.ID, sizeName)

		// Create thumbnail
		thumbnail := s.imageProcessor.CreateThumbnail(img, maxSize)

		// Save thumbnail to storage
		writer, err := b.NewWriter(ctx, thumbnailPath, nil)
		if err != nil {
			return errkit.Wrap(err, "failed to create thumbnail writer")
		}

		// Always encode thumbnails as JPEG for consistency and smaller file sizes
		err = jpeg.Encode(writer, thumbnail, &jpeg.Options{Quality: 90})

		writer.Close()
		if err != nil {
			return errkit.Wrap(err, "failed to encode thumbnail")
		}
	}

	return nil
}

// getThumbnailPath generates the thumbnail file path using file ID
// All thumbnails are saved as JPEG files regardless of the original format
func (s *FileService) getThumbnailPath(fileID, sizeName string) string {
	// Use file ID for thumbnail paths to avoid conflicts with user-controlled paths
	return fmt.Sprintf("thumbnails/%s_%s.jpg", fileID, sizeName)
}

// GetThumbnailPaths returns the paths of all thumbnails for a given file ID
func (s *FileService) GetThumbnailPaths(fileID string) map[string]string {
	thumbnails := make(map[string]string)
	for sizeName := range s.thumbnailSizes {
		thumbnails[sizeName] = s.getThumbnailPath(fileID, sizeName)
	}
	return thumbnails
}

// DeleteFileWithPhysical deletes a file entity and its associated physical file
func (s *FileService) DeleteFileWithPhysical(ctx context.Context, fileID string) error {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to create file registry")
	}

	// Get the file entity first
	file, err := fileReg.Get(ctx, fileID)
	if err != nil {
		return errkit.Wrap(err, "failed to get file entity")
	}

	// Delete the physical file and thumbnails if they exist
	if file.File != nil && file.File.OriginalPath != "" {
		if err := s.deletePhysicalFileAndThumbnails(ctx, fileID, file.File.OriginalPath, file.File.MIMEType); err != nil {
			return errkit.Wrap(err, "failed to delete physical file and thumbnails")
		}
	}

	// Delete the file entity from database
	err = fileReg.Delete(ctx, fileID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete file entity")
	}

	return nil
}

// DeletePhysicalFile deletes only the physical file from storage
func (s *FileService) DeletePhysicalFile(ctx context.Context, filePath string) error {
	return s.deletePhysicalFile(ctx, filePath)
}

// deletePhysicalFileAndThumbnails deletes the physical file and all its thumbnails
func (s *FileService) deletePhysicalFileAndThumbnails(ctx context.Context, fileID, filePath, mimeType string) error {
	// Delete the original file
	if err := s.deletePhysicalFile(ctx, filePath); err != nil {
		return errkit.Wrap(err, "failed to delete original file")
	}

	// Delete thumbnails if it's an image file
	if mimekit.IsImage(mimeType) {
		thumbnailPaths := s.GetThumbnailPaths(fileID)
		for _, thumbnailPath := range thumbnailPaths {
			// Don't fail if thumbnail doesn't exist - it might not have been generated
			_ = s.deletePhysicalFile(ctx, thumbnailPath)
		}
	}

	return nil
}

// deletePhysicalFile is the unified implementation for deleting physical files
func (s *FileService) deletePhysicalFile(ctx context.Context, filePath string) error {
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open bucket")
	}
	defer b.Close()

	// Check if file exists before trying to delete it
	exists, err := b.Exists(ctx, filePath)
	if err != nil {
		return errkit.Wrap(err, "failed to check if file exists")
	}

	if !exists {
		// File doesn't exist, nothing to delete - this is not an error
		return nil
	}

	err = b.Delete(ctx, filePath)
	if err != nil {
		return errkit.Wrap(err, "failed to delete file")
	}

	return nil
}

// DeleteLinkedFiles deletes all files linked to a specific entity
func (s *FileService) DeleteLinkedFiles(ctx context.Context, entityType, entityID string) error {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to create file registry")
	}

	// Get all linked files for this entity
	files, err := fileReg.ListByLinkedEntity(ctx, entityType, entityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get linked files")
	}

	// Delete all linked files (both physical and database records)
	for _, file := range files {
		err = s.DeleteFileWithPhysical(ctx, file.ID)
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errkit.Wrap(err, "failed to delete linked file")
		}
	}

	return nil
}

// ThumbnailExists checks if a thumbnail file exists for a given file and size
func (s *FileService) ThumbnailExists(ctx context.Context, fileID, size string) (bool, error) {
	// Validate size parameter
	if size != "small" && size != "medium" {
		return false, errkit.WithStack(ErrInvalidThumbnailSize, "size", size)
	}

	// Generate thumbnail path using the same structure as download
	thumbnailPath := s.getThumbnailPath(fileID, size)

	// Open bucket and check if file exists
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return false, errkit.Wrap(err, "failed to open bucket")
	}
	defer b.Close()

	exists, err := b.Exists(ctx, thumbnailPath)
	if err != nil {
		return false, errkit.Wrap(err, "failed to check thumbnail existence").WithField("thumbnail_path", thumbnailPath)
	}

	return exists, nil
}
