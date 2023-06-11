package registry

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type AreaRegistry = Registry[models.Area]

type baseMemoryAreaRegistry = MemoryRegistry[models.Area]
type MemoryAreaRegistry struct {
	*baseMemoryAreaRegistry

	locationRegistry LocationRegistry
}

func NewMemoryAreaRegistry(locationRegistry LocationRegistry) *MemoryAreaRegistry {
	return &MemoryAreaRegistry{
		baseMemoryAreaRegistry: NewMemoryRegistry[models.Area](),
		locationRegistry:       locationRegistry,
	}
}

func (r *MemoryAreaRegistry) Create(area models.Area) (*models.Area, error) {
	err := validation.Validate(area)
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
