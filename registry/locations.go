package registry

import (
	"sync"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type LocationRegistry interface {
	Registry[models.Location]

	AddArea(locationID, areaID string)
	GetAreas(locationID string) []string
	DeleteArea(locationID, areaID string)
}

type baseMemoryLocationRegistry = MemoryRegistry[models.Location]
type MemoryLocationRegistry struct {
	*baseMemoryLocationRegistry

	areasLock sync.RWMutex
	areas     models.LocationAreas
}

func NewMemoryLocationRegistry() *MemoryLocationRegistry {
	return &MemoryLocationRegistry{
		baseMemoryLocationRegistry: NewMemoryRegistry[models.Location](),
		areas:                      make(models.LocationAreas),
	}
}

func (r *MemoryLocationRegistry) Delete(id string) error {
	_, err := r.baseMemoryLocationRegistry.Get(id)
	if err != nil {
		return err
	}

	if len(r.GetAreas(id)) > 0 {
		return errkit.Wrap(ErrCannotDelete, "location has areas")
	}

	err = r.baseMemoryLocationRegistry.Delete(id)
	if err != nil {
		return err
	}

	return nil
}

func (r *MemoryLocationRegistry) AddArea(locationID, areaID string) {
	r.areasLock.Lock()
	r.areas[locationID] = append(r.areas[locationID], areaID)
	r.areasLock.Unlock()
}

func (r *MemoryLocationRegistry) GetAreas(locationID string) []string {
	r.areasLock.RLock()
	areas := make([]string, len(r.areas[locationID]))
	copy(areas, r.areas[locationID])
	r.areasLock.RUnlock()

	return areas
}

func (r *MemoryLocationRegistry) DeleteArea(locationID, areaID string) {
	r.areasLock.Lock()
	for i, foundAreaID := range r.areas[locationID] {
		if foundAreaID == areaID {
			r.areas[locationID] = append(r.areas[locationID][:i], r.areas[locationID][i+1:]...)
			break
		}
	}
	r.areasLock.Unlock()
}
