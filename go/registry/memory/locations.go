package memory

import (
	"context"
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

	areasLock sync.RWMutex
	areas     models.LocationAreas
}

func NewLocationRegistry() *LocationRegistry {
	return &LocationRegistry{
		baseLocationRegistry: NewRegistry[models.Location, *models.Location](),
		areas:                make(models.LocationAreas),
	}
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
