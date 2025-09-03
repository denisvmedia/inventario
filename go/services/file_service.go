package services

import (
	"context"
	"errors"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry"
)

// FileService provides business logic for file operations
type FileService struct {
	factorySet     *registry.FactorySet
	uploadLocation string
}

// NewFileService creates a new file service
func NewFileService(factorySet *registry.FactorySet, uploadLocation string) *FileService {
	return &FileService{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
	}
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

	// Delete the physical file first if it exists
	if file.File != nil && file.File.OriginalPath != "" {
		if err := s.deletePhysicalFile(ctx, file.File.OriginalPath); err != nil {
			return errkit.Wrap(err, "failed to delete physical file")
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
