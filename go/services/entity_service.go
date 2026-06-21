package services

import (
	"context"
	"errors"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

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

// DeleteCommodityRecursive deletes a commodity and all its linked files.
//
// Ordering is row-first, blob-best-effort (#2120), mirroring
// DeleteExportWithFile: the linked file IDs are collected first (no blob
// deletes yet), then the commodity row is dropped — its children
// (loans/services/supply-links) CASCADE and its cover_file_id FK is
// ON DELETE SET NULL, so the row delete never trips a constraint — and only
// then are the previously-linked files removed (row + best-effort blob) via
// DeleteFileWithPhysical. Deleting the commodity before the files avoids the
// old failure mode where a blob error aborted the operation after some rows
// were already gone.
func (s *EntityService) DeleteCommodityRecursive(ctx context.Context, id string) error {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create file registry", err)
	}

	// Collect the IDs of the files linked to this commodity before it is
	// deleted. We only need the IDs here — the actual file (row + blob)
	// deletion happens after the commodity row is gone.
	linkedFiles, err := fileReg.ListByLinkedEntity(ctx, "commodity", id)
	if err != nil {
		return errxtrace.Wrap("failed to list linked files", err)
	}
	fileIDs := make([]string, 0, len(linkedFiles))
	for _, file := range linkedFiles {
		fileIDs = append(fileIDs, file.ID)
	}

	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create commodity registry", err)
	}

	// Delete the commodity row first. Children CASCADE; cover_file_id is
	// SET NULL, so the linked files can still be removed afterwards.
	if err := comReg.Delete(ctx, id); err != nil {
		return errxtrace.Wrap("failed to delete commodity", err)
	}

	// The commodity row has been removed. Now delete the linked files
	// (row + best-effort blob). A file that is already gone is fine.
	for _, fileID := range fileIDs {
		if err := s.fileService.DeleteFileWithPhysical(ctx, fileID); err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Wrap("failed to delete linked file", err, errx.Attrs("file_id", fileID))
		}
	}

	return nil
}

// DeleteAreaRecursive deletes an area and all its commodities recursively
func (s *EntityService) DeleteAreaRecursive(ctx context.Context, id string) error {
	areaReg, err := s.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create area registry", err)
	}

	// Check if area exists first - if it's already deleted, that's fine
	_, err = areaReg.Get(ctx, id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			// Area is already deleted, nothing to do
			return nil
		}
		return errxtrace.Wrap("failed to get area", err)
	}

	// Get all commodities in this area first
	commodities, err := areaReg.GetCommodities(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get commodities", err)
	}

	// Delete all commodities recursively (this will also delete their files)
	for _, commodityID := range commodities {
		if err := s.DeleteCommodityRecursive(ctx, commodityID); err != nil {
			// If the commodity is already deleted, that's fine - continue with others
			if !errors.Is(err, registry.ErrNotFound) {
				return errxtrace.Wrap("failed to delete commodity recursively", err, errx.Attrs("commodity_id", commodityID))
			}
		}
	}

	// Delete files attached directly to the area (#2119) before the area row
	// is removed, mirroring the commodity path. ErrNotFound is tolerated so a
	// concurrently-removed file doesn't abort the cascade.
	if err := s.fileService.DeleteLinkedFiles(ctx, "area", id); err != nil && !errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Wrap("failed to delete area files", err, errx.Attrs("areaID", id))
	}

	// Finally delete the area itself
	return areaReg.Delete(ctx, id)
}

// DeleteLocationRecursive deletes a location and all its areas and commodities recursively
func (s *EntityService) DeleteLocationRecursive(ctx context.Context, id string) error {
	locReg, err := s.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create location registry", err)
	}

	// Get the location to ensure it exists
	_, err = locReg.Get(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get location", err)
	}

	// Get all areas in this location
	areas, err := locReg.GetAreas(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get areas", err)
	}

	// Delete all areas recursively (this will also delete their commodities)
	for _, areaID := range areas {
		if err := s.DeleteAreaRecursive(ctx, areaID); err != nil {
			// If the area is already deleted, that's fine - continue with others
			if !errors.Is(err, registry.ErrNotFound) {
				return errxtrace.Wrap("failed to delete area recursively", err, errx.Attrs("areaID", areaID))
			}
		}
	}

	// Delete files attached directly to the location (#2119) before the
	// location row is removed, mirroring the commodity path. ErrNotFound is
	// tolerated so a concurrently-removed file doesn't abort the cascade.
	if err := s.fileService.DeleteLinkedFiles(ctx, "location", id); err != nil && !errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Wrap("failed to delete location files", err, errx.Attrs("locationID", id))
	}

	// Finally delete the location itself
	return locReg.Delete(ctx, id)
}

// DeleteArea deletes an EMPTY area together with the files attached directly to
// the area (#2119). It is NON-recursive on purpose: the underlying registry
// Delete returns ErrCannotDelete while the area still holds commodities, so a
// non-empty area is rejected (HTTP 422) before anything is removed — preserving
// the long-standing "can't delete a non-empty area" guard. Letting the user
// choose cascade-vs-unlink for a non-empty area is a separate feature.
func (s *EntityService) DeleteArea(ctx context.Context, id string) error {
	areaReg, err := s.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create area registry", err)
	}

	// Delete the area row first: this returns ErrCannotDelete when the area
	// still has commodities, so a non-empty area is left fully intact.
	if err := areaReg.Delete(ctx, id); err != nil {
		return errxtrace.Wrap("failed to delete area", err)
	}

	// The (empty) area row is gone; remove files attached directly to the area
	// so they don't orphan. A concurrently-removed file is fine.
	if err := s.fileService.DeleteLinkedFiles(ctx, "area", id); err != nil && !errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Wrap("failed to delete area files", err, errx.Attrs("areaID", id))
	}

	return nil
}

// DeleteLocation deletes an EMPTY location together with the files attached
// directly to the location (#2119). Non-recursive, mirroring DeleteArea: the
// registry Delete returns ErrCannotDelete while the location still holds areas,
// so a non-empty location is rejected (HTTP 422) before anything is removed.
func (s *EntityService) DeleteLocation(ctx context.Context, id string) error {
	locReg, err := s.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create location registry", err)
	}

	// Delete the location row first: ErrCannotDelete leaves a non-empty
	// location intact.
	if err := locReg.Delete(ctx, id); err != nil {
		return errxtrace.Wrap("failed to delete location", err)
	}

	// The (empty) location row is gone; remove files attached directly to it.
	if err := s.fileService.DeleteLinkedFiles(ctx, "location", id); err != nil && !errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Wrap("failed to delete location files", err, errx.Attrs("locationID", id))
	}

	return nil
}

// DeleteExportWithFile deletes an export and its associated file
func (s *EntityService) DeleteExportWithFile(ctx context.Context, id string) error {
	expReg, err := s.factorySet.ExportRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create export registry", err)
	}

	// Get the export to check if it has a linked file
	export, err := expReg.Get(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get export", err)
	}

	// Store file ID for deletion after export is deleted
	var fileIDToDelete *string
	if export.FileID != nil && *export.FileID != "" {
		fileIDToDelete = export.FileID
	}

	// Delete the export first to avoid foreign key constraint violation
	err = expReg.Delete(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to delete export", err)
	}

	// Then delete the associated file if it exists
	if fileIDToDelete != nil {
		err = s.fileService.DeleteFileWithPhysical(ctx, *fileIDToDelete)
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Wrap("failed to delete export file", err)
		}
	}

	return nil
}
