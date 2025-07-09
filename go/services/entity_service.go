package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry"
)

// EntityService provides business logic for entity operations including recursive deletion
type EntityService struct {
	registrySet *registry.Set
	fileService *FileService
}

// NewEntityService creates a new entity service
func NewEntityService(registrySet *registry.Set, uploadLocation string) *EntityService {
	return &EntityService{
		registrySet: registrySet,
		fileService: NewFileService(registrySet, uploadLocation),
	}
}

// DeleteCommodityRecursive deletes a commodity and all its linked files recursively
func (s *EntityService) DeleteCommodityRecursive(ctx context.Context, id string) error {
	// Delete all linked files (both physical and database records)
	err := s.fileService.DeleteLinkedFiles(ctx, "commodity", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete linked files")
	}

	// Then delete the commodity itself
	return s.registrySet.CommodityRegistry.Delete(ctx, id)
}

// DeleteAreaRecursive deletes an area and all its commodities recursively
func (s *EntityService) DeleteAreaRecursive(ctx context.Context, id string) error {
	// Check if area exists first - if it's already deleted, that's fine
	_, err := s.registrySet.AreaRegistry.Get(ctx, id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			// Area is already deleted, nothing to do
			return nil
		}
		return errkit.Wrap(err, "failed to get area")
	}

	// Get all commodities in this area first
	commodities, err := s.registrySet.AreaRegistry.GetCommodities(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodities")
	}

	// Delete all commodities recursively (this will also delete their files)
	for _, commodityID := range commodities {
		if err := s.DeleteCommodityRecursive(ctx, commodityID); err != nil {
			// If the commodity is already deleted, that's fine - continue with others
			if !errors.Is(err, registry.ErrNotFound) {
				return errkit.Wrap(err, fmt.Sprintf("failed to delete commodity %s recursively", commodityID))
			}
		}
	}

	// Finally delete the area itself
	return s.registrySet.AreaRegistry.Delete(ctx, id)
}

// DeleteLocationRecursive deletes a location and all its areas and commodities recursively
func (s *EntityService) DeleteLocationRecursive(ctx context.Context, id string) error {
	// Get the location to ensure it exists
	_, err := s.registrySet.LocationRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get location")
	}

	// Get all areas in this location
	areas, err := s.registrySet.LocationRegistry.GetAreas(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get areas")
	}

	// Delete all areas recursively (this will also delete their commodities)
	for _, areaID := range areas {
		if err := s.DeleteAreaRecursive(ctx, areaID); err != nil {
			// If the area is already deleted, that's fine - continue with others
			if !errors.Is(err, registry.ErrNotFound) {
				return errkit.Wrap(err, fmt.Sprintf("failed to delete area %s recursively", areaID))
			}
		}
	}

	// Finally delete the location itself
	return s.registrySet.LocationRegistry.Delete(ctx, id)
}

// DeleteExportWithFile deletes an export and its associated file
func (s *EntityService) DeleteExportWithFile(ctx context.Context, id string) error {
	// Get the export to check if it has a linked file
	export, err := s.registrySet.ExportRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get export")
	}

	// If export has a linked file, delete it (both physical and database record)
	if export.FileID != "" {
		err = s.fileService.DeleteFileWithPhysical(ctx, export.FileID)
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errkit.Wrap(err, "failed to delete export file")
		}
	}

	// Then delete the export itself
	return s.registrySet.ExportRegistry.Delete(ctx, id)
}
