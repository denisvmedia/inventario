package memory

import (
	"sync"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type baseAreaRegistry = Registry[models.Area]

type AreaRegistry struct {
	*baseAreaRegistry

	locationRegistry registry.LocationRegistry
	commoditiesLock  sync.RWMutex
	commodities      models.AreaCommodities
}

func NewAreaRegistry(locationRegistry registry.LocationRegistry) *AreaRegistry {
	return &AreaRegistry{
		baseAreaRegistry: NewRegistry[models.Area](),
		locationRegistry: locationRegistry,
		commodities:      make(models.AreaCommodities),
	}
}

func (r *AreaRegistry) Create(area models.Area) (*models.Area, error) {
	err := validation.Validate(&area)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.locationRegistry.Get(area.LocationID)
	if err != nil {
		return nil, errkit.Wrap(err, "location not found")
	}

	newArea, err := r.baseAreaRegistry.Create(area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create area")
	}

	r.locationRegistry.AddArea(area.LocationID, newArea.ID)

	return newArea, err
}

func (r *AreaRegistry) Delete(id string) error {
	area, err := r.baseAreaRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseAreaRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.locationRegistry.DeleteArea(area.LocationID, id)

	return nil
}

func (r *AreaRegistry) AddCommodity(areaID, commodityID string) {
	r.commoditiesLock.Lock()
	r.commodities[areaID] = append(r.commodities[areaID], commodityID)
	r.commoditiesLock.Unlock()
}

func (r *AreaRegistry) GetCommodities(areaID string) []string {
	r.commoditiesLock.RLock()
	commodities := make([]string, len(r.commodities[areaID]))
	copy(commodities, r.commodities[areaID])
	r.commoditiesLock.RUnlock()

	return commodities
}

func (r *AreaRegistry) DeleteCommodity(areaID, commodityID string) {
	r.commoditiesLock.Lock()
	for i, foundCommodityID := range r.commodities[areaID] {
		if foundCommodityID == commodityID {
			r.commodities[areaID] = append(r.commodities[areaID][:i], r.commodities[areaID][i+1:]...)
			break
		}
	}
	r.commoditiesLock.Unlock()
}
