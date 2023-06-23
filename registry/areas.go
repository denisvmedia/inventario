package registry

import (
	"sync"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type AreaRegistry interface {
	Registry[models.Area]

	AddCommodity(areaID, commodityID string)
	GetCommodities(areaID string) []string
	DeleteCommodity(areaID, commodityID string)
}

type baseMemoryAreaRegistry = MemoryRegistry[models.Area]
type MemoryAreaRegistry struct {
	*baseMemoryAreaRegistry

	locationRegistry LocationRegistry
	commoditiesLock  sync.RWMutex
	commodities      models.AreaCommodities
}

func NewMemoryAreaRegistry(locationRegistry LocationRegistry) *MemoryAreaRegistry {
	return &MemoryAreaRegistry{
		baseMemoryAreaRegistry: NewMemoryRegistry[models.Area](),
		locationRegistry:       locationRegistry,
		commodities:            make(models.AreaCommodities),
	}
}

func (r *MemoryAreaRegistry) Create(area models.Area) (*models.Area, error) {
	err := validation.Validate(&area)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.locationRegistry.Get(area.LocationID)
	if err != nil {
		return nil, errkit.Wrap(err, "location not found")
	}

	newArea, err := r.baseMemoryAreaRegistry.Create(area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create area")
	}

	r.locationRegistry.AddArea(area.LocationID, newArea.ID)

	return newArea, err
}

func (r *MemoryAreaRegistry) Delete(id string) error {
	area, err := r.baseMemoryAreaRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseMemoryAreaRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.locationRegistry.DeleteArea(area.LocationID, id)

	return nil
}

func (r *MemoryAreaRegistry) AddCommodity(areaID, commodityID string) {
	r.commoditiesLock.Lock()
	r.commodities[areaID] = append(r.commodities[areaID], commodityID)
	r.commoditiesLock.Unlock()
}

func (r *MemoryAreaRegistry) GetCommodities(areaID string) []string {
	r.commoditiesLock.RLock()
	commodities := make([]string, len(r.commodities[areaID]))
	copy(commodities, r.commodities[areaID])
	r.commoditiesLock.RUnlock()

	return commodities
}

func (r *MemoryAreaRegistry) DeleteCommodity(areaID, commodityID string) {
	r.commoditiesLock.Lock()
	for i, foundCommodityID := range r.commodities[areaID] {
		if foundCommodityID == commodityID {
			r.commodities[areaID] = append(r.commodities[areaID][:i], r.commodities[areaID][i+1:]...)
			break
		}
	}
	r.commoditiesLock.Unlock()
}
