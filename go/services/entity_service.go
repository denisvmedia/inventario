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
	factorySet  *registry.FactorySet
	fileService *FileService
}

// NewEntityService creates a new entity service
func NewEntityService(factorySet *registry.FactorySet, uploadLocation string) *EntityService {
	return &EntityService{
		factorySet:  factorySet,
		fileService: NewFileService(factorySet, uploadLocation),
	}
}

// DeleteCommodityRecursive deletes a commodity and all its linked files recursively
func (s *EntityService) DeleteCommodityRecursive(ctx context.Context, id string) error {
	// Delete all linked files (both physical and database records)
	err := s.fileService.DeleteLinkedFiles(ctx, "commodity", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete linked files")
	}

	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to create commodity registry")
	}

	// Then delete the commodity itself
	return comReg.Delete(ctx, id)
}

// DeleteAreaRecursive deletes an area and all its commodities recursively
func (s *EntityService) DeleteAreaRecursive(ctx context.Context, id string) error {
	areaReg, err := s.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to create area registry")
	}

	// Check if area exists first - if it's already deleted, that's fine
	_, err = areaReg.Get(ctx, id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			// Area is already deleted, nothing to do
			return nil
		}
		return errkit.Wrap(err, "failed to get area")
	}

	// Get all commodities in this area first
	commodities, err := areaReg.GetCommodities(ctx, id)
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
	return areaReg.Delete(ctx, id)
}

// DeleteLocationRecursive deletes a location and all its areas and commodities recursively
func (s *EntityService) DeleteLocationRecursive(ctx context.Context, id string) error {
	locReg, err := s.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to create location registry")
	}

	// Get the location to ensure it exists
	_, err = locReg.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get location")
	}

	// Get all areas in this location
	areas, err := locReg.GetAreas(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get areas")
	}

	// Delete all areas recursively (this will also delete their commodities)
	for _, areaID := range areas {
		if err := s.DeleteAreaRecursive(ctx, areaID); err != nil {
			// If the area is already deleted, that's fine - continue with others
			if !errors.Is(err, registry.ErrNotFound) {
				return errkit.Wrap(err, "failed to delete area recursively", "areaID", areaID)
			}
		}
	}

	// Finally delete the location itself
	return locReg.Delete(ctx, id)
}

// DeleteExportWithFile deletes an export and its associated file
func (s *EntityService) DeleteExportWithFile(ctx context.Context, id string) error {
	expReg, err := s.factorySet.ExportRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to create export registry")
	}

	// Get the export to check if it has a linked file
	export, err := expReg.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get export")
	}

	// Store file ID for deletion after export is deleted
	var fileIDToDelete *string
	if export.FileID != nil && *export.FileID != "" {
		fileIDToDelete = export.FileID
	}

	// Delete the export first to avoid foreign key constraint violation
	err = expReg.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete export")
	}

	// Then delete the associated file if it exists
	if fileIDToDelete != nil {
		err = s.fileService.DeleteFileWithPhysical(ctx, *fileIDToDelete)
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errkit.Wrap(err, "failed to delete export file")
		}
	}

	return nil
}
