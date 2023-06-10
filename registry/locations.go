package registry

import (
	"sync"

	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/internal/orderedmap"
	"github.com/denisvmedia/inventario/models"
)

type LocationRegistry interface {
	// Create creates a new location in the registry.
	Create(location models.Location) (*models.Location, error)

	// Get returns a location from the registry.
	Get(id string) (*models.Location, error)

	// List returns a list of locations from the registry.
	List() ([]models.Location, error)

	// Update updates a location in the registry.
	Update(location models.Location) (*models.Location, error)

	// Delete deletes a location from the registry.
	Delete(id string) error

	// Count returns the number of locations in the registry.
	Count() (int, error)
}

type MemoryLocationRegistry struct {
	locations        *orderedmap.OrderedMap[models.Location] // map[string]models.Location
	locationsOrdered []string
	lock             sync.RWMutex
}

func NewMemoryLocationRegistry() *MemoryLocationRegistry {
	return &MemoryLocationRegistry{
		locations: orderedmap.New[models.Location](),
	}
}

func (r *MemoryLocationRegistry) Create(location models.Location) (*models.Location, error) {
	location.ID = uuid.New().String()
	r.locations.Set(location.ID, location)

	return &location, nil
}

func (r *MemoryLocationRegistry) Get(id string) (*models.Location, error) {
	location, ok := r.locations.Get(id)
	if !ok {
		return nil, ErrNotFound
	}
	return &location, nil
}

func (r *MemoryLocationRegistry) List() ([]models.Location, error) {
	locations := make([]models.Location, 0, r.locations.Len())
	for _, location := range r.locations.Iterate() {
		locations = append(locations, location.Value)
	}
	return locations, nil
}

func (r *MemoryLocationRegistry) Update(location models.Location) (*models.Location, error) {
	if _, ok := r.locations.Get(location.ID); !ok {
		return nil, ErrNotFound
	}

	r.locations.Set(location.ID, location)
	return &location, nil
}

func (r *MemoryLocationRegistry) Delete(id string) error {
	r.locations.Delete(id)
	return nil
}

func (r *MemoryLocationRegistry) Count() (int, error) {
	return r.locations.Len(), nil
}
