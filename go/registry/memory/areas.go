package memory

import (
	"sync"

	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"

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

	err = r.locationRegistry.AddArea(area.LocationID, newArea.ID)

	return newArea, err
}

func (r *AreaRegistry) Delete(id string) error {
	area, err := r.baseAreaRegistry.Get(id)
	if err != nil {
		return err
	}

	if len(must.Must(r.GetCommodities(id))) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "area has commodities")
	}

	err = r.baseAreaRegistry.Delete(id)
	if err != nil {
		return err
	}

	err = r.locationRegistry.DeleteArea(area.LocationID, id)

	return err
}

func (r *AreaRegistry) AddCommodity(areaID, commodityID string) error {
	r.commoditiesLock.Lock()
	r.commodities[areaID] = append(r.commodities[areaID], commodityID)
	r.commoditiesLock.Unlock()

	return nil
}

func (r *AreaRegistry) GetCommodities(areaID string) ([]string, error) {
	r.commoditiesLock.RLock()
	commodities := make([]string, len(r.commodities[areaID]))
	copy(commodities, r.commodities[areaID])
	r.commoditiesLock.RUnlock()

	return commodities, nil
}

func (r *AreaRegistry) DeleteCommodity(areaID, commodityID string) error {
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
