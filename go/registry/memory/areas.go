package memory

import (
	"context"
	"strings"
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

	locationRegistry registry.LocationRegistry
	commoditiesLock  sync.RWMutex
	commodities      models.AreaCommodities
}

func NewAreaRegistry(locationRegistry registry.LocationRegistry) *AreaRegistry {
	return &AreaRegistry{
		baseAreaRegistry: NewRegistry[models.Area, *models.Area](),
		locationRegistry: locationRegistry,
		commodities:      make(models.AreaCommodities),
	}
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

// Enhanced methods with simplified in-memory implementations

// GetCommodityCount returns the number of commodities in an area (simplified)
func (r *AreaRegistry) GetCommodityCount(ctx context.Context, areaID string) (int, error) {
	commodities, err := r.GetCommodities(ctx, areaID)
	if err != nil {
		return 0, err
	}
	return len(commodities), nil
}

// GetTotalValue calculates the total value of commodities in an area (simplified)
func (r *AreaRegistry) GetTotalValue(ctx context.Context, areaID string, currency string) (float64, error) {
	// This is a simplified implementation that would require access to commodity data
	// In a real implementation, this would need to be coordinated with the commodity registry
	// For now, return 0 as a placeholder
	return 0.0, nil
}

// SearchByName searches areas by name using simple text matching (simplified)
func (r *AreaRegistry) SearchByName(ctx context.Context, query string) ([]*models.Area, error) {
	areas, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []*models.Area

	for _, area := range areas {
		if strings.Contains(strings.ToLower(area.Name), query) {
			filtered = append(filtered, area)
		}
	}

	return filtered, nil
}
