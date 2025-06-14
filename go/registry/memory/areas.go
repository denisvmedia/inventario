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

var _ registry.AreaRegistry = (*AreaRegistry)(nil)

type baseAreaRegistry = Registry[models.Area, *models.Area]

type AreaRegistry struct {
	*baseAreaRegistry

	locationRegistry  registry.LocationRegistry
	commodityRegistry registry.CommodityRegistry
	commoditiesLock   sync.RWMutex
	commodities       models.AreaCommodities
}

func NewAreaRegistry(locationRegistry registry.LocationRegistry) *AreaRegistry {
	return &AreaRegistry{
		baseAreaRegistry: NewRegistry[models.Area, *models.Area](),
		locationRegistry: locationRegistry,
		commodities:      make(models.AreaCommodities),
	}
}

// SetCommodityRegistry sets the commodity registry for recursive deletion
func (r *AreaRegistry) SetCommodityRegistry(commodityRegistry registry.CommodityRegistry) {
	r.commodityRegistry = commodityRegistry
}

func (r *AreaRegistry) Create(ctx context.Context, area models.Area) (*models.Area, error) {
	_, err := r.locationRegistry.Get(ctx, area.LocationID)
	if err != nil {
		return nil, errkit.Wrap(err, "location not found")
	}

	newArea, err := r.baseAreaRegistry.Create(ctx, area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create area")
	}

	err = r.locationRegistry.AddArea(ctx, area.LocationID, newArea.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to add area to location")
	}

	return newArea, nil
}

func (r *AreaRegistry) Delete(ctx context.Context, id string) error {
	area, err := r.baseAreaRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get area")
	}

	if len(must.Must(r.GetCommodities(ctx, id))) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "area has commodities")
	}

	err = r.baseAreaRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area")
	}

	err = r.locationRegistry.DeleteArea(ctx, area.LocationID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area from location")
	}

	return nil
}

// DeleteRecursive deletes an area and all its commodities recursively
func (r *AreaRegistry) DeleteRecursive(ctx context.Context, id string) error {
	// Get the area to ensure it exists - if it's already deleted, that's fine
	area, err := r.baseAreaRegistry.Get(ctx, id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			// Area is already deleted, nothing to do
			return nil
		}
		return errkit.Wrap(err, "failed to get area")
	}

	// Get all commodities in this area
	commodities, err := r.GetCommodities(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodities")
	}

	// Delete all commodities
	for _, commodityID := range commodities {
		if r.commodityRegistry != nil {
			if err := r.commodityRegistry.Delete(ctx, commodityID); err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to delete commodity %s", commodityID))
			}
		}
	}

	// Now delete the area itself
	err = r.baseAreaRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area")
	}

	// Remove area from location
	err = r.locationRegistry.DeleteArea(ctx, area.LocationID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area from location")
	}

	// Clean up the commodities mapping
	r.commoditiesLock.Lock()
	delete(r.commodities, id)
	r.commoditiesLock.Unlock()

	return nil
}

func (r *AreaRegistry) AddCommodity(_ context.Context, areaID, commodityID string) error {
	r.commoditiesLock.Lock()
	r.commodities[areaID] = append(r.commodities[areaID], commodityID)
	r.commoditiesLock.Unlock()

	return nil
}

func (r *AreaRegistry) GetCommodities(_ context.Context, areaID string) ([]string, error) {
	r.commoditiesLock.RLock()
	commodities := make([]string, len(r.commodities[areaID]))
	copy(commodities, r.commodities[areaID])
	r.commoditiesLock.RUnlock()

	return commodities, nil
}

func (r *AreaRegistry) DeleteCommodity(_ context.Context, areaID, commodityID string) error {
	r.commoditiesLock.Lock()
	for i, foundCommodityID := range r.commodities[areaID] {
		if foundCommodityID == commodityID {
			r.commodities[areaID] = append(r.commodities[areaID][:i], r.commodities[areaID][i+1:]...)
			break
		}
	}
	r.commoditiesLock.Unlock()

	return nil
}
