package memory

import (
	"sync"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type baseLocationRegistry = Registry[models.Location]
type LocationRegistry struct {
	*baseLocationRegistry

	areasLock sync.RWMutex
	areas     models.LocationAreas
}

func NewLocationRegistry() *LocationRegistry {
	return &LocationRegistry{
		baseLocationRegistry: NewRegistry[models.Location](),
		areas:                make(models.LocationAreas),
	}
}

func (r *LocationRegistry) Delete(id string) error {
	_, err := r.baseLocationRegistry.Get(id)
	if err != nil {
		return err
	}

	if len(r.GetAreas(id)) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "location has areas")
	}

	err = r.baseLocationRegistry.Delete(id)
	if err != nil {
		return err
	}

	return nil
}

func (r *LocationRegistry) AddArea(locationID, areaID string) {
	r.areasLock.Lock()
	r.areas[locationID] = append(r.areas[locationID], areaID)
	r.areasLock.Unlock()
}

func (r *LocationRegistry) GetAreas(locationID string) []string {
	r.areasLock.RLock()
	areas := make([]string, len(r.areas[locationID]))
	copy(areas, r.areas[locationID])
	r.areasLock.RUnlock()

	return areas
}

func (r *LocationRegistry) DeleteArea(locationID, areaID string) {
	r.areasLock.Lock()
	for i, foundAreaID := range r.areas[locationID] {
		if foundAreaID == areaID {
			r.areas[locationID] = append(r.areas[locationID][:i], r.areas[locationID][i+1:]...)
			break
		}
	}
	r.areasLock.Unlock()
}
