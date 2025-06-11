package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.LocationRegistry = (*LocationRegistry)(nil)

type baseLocationRegistry = Registry[models.Location, *models.Location]
type LocationRegistry struct {
	*baseLocationRegistry

	areasLock    sync.RWMutex
	areas        models.LocationAreas
	areaRegistry registry.AreaRegistry
}

func NewLocationRegistry() *LocationRegistry {
	return &LocationRegistry{
		baseLocationRegistry: NewRegistry[models.Location, *models.Location](),
		areas:                make(models.LocationAreas),
	}
}

// SetAreaRegistry sets the area registry for recursive deletion
func (r *LocationRegistry) SetAreaRegistry(areaRegistry registry.AreaRegistry) {
	r.areaRegistry = areaRegistry
}

func (r *LocationRegistry) Delete(ctx context.Context, id string) error {
	_, err := r.baseLocationRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get location")
	}

	if len(must.Must(r.GetAreas(ctx, id))) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "location has areas")
	}

	err = r.baseLocationRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete location")
	}

	return nil
}

func (r *LocationRegistry) AddArea(_ context.Context, locationID, areaID string) error {
	r.areasLock.Lock()
	r.areas[locationID] = append(r.areas[locationID], areaID)
	r.areasLock.Unlock()

	return nil
}

func (r *LocationRegistry) GetAreas(_ context.Context, locationID string) ([]string, error) {
	r.areasLock.RLock()
	areas := make([]string, len(r.areas[locationID]))
	copy(areas, r.areas[locationID])
	r.areasLock.RUnlock()

	return areas, nil
}

func (r *LocationRegistry) DeleteArea(_ context.Context, locationID, areaID string) error {
	r.areasLock.Lock()
	for i, foundAreaID := range r.areas[locationID] {
		if foundAreaID == areaID {
			r.areas[locationID] = append(r.areas[locationID][:i], r.areas[locationID][i+1:]...)
			break
		}
	}
	r.areasLock.Unlock()

	return nil
}

// DeleteRecursive deletes a location and all its areas and commodities recursively
func (r *LocationRegistry) DeleteRecursive(ctx context.Context, id string) error {
	// Get the location to ensure it exists
	_, err := r.baseLocationRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get location")
	}

	// Get all areas in this location
	areas, err := r.GetAreas(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get areas")
	}

	// Delete all areas recursively (this will also delete their commodities)
	for _, areaID := range areas {
		// We need access to the area registry to delete areas recursively
		// This will be injected via constructor or setter
		if r.areaRegistry != nil {
			if err := r.areaRegistry.DeleteRecursive(ctx, areaID); err != nil {
				// If the area is already deleted, that's fine - continue with others
				if !errors.Is(err, registry.ErrNotFound) {
					return errkit.Wrap(err, fmt.Sprintf("failed to delete area %s recursively", areaID))
				}
			}
		}
	}

	// Now delete the location itself
	err = r.baseLocationRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete location")
	}

	// Clean up the areas mapping
	r.areasLock.Lock()
	delete(r.areas, id)
	r.areasLock.Unlock()

	return nil
}
