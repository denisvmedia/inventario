package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// LocationRegistryFactory creates LocationRegistry instances with proper context
type LocationRegistryFactory struct {
	baseLocationRegistry *Registry[models.Location, *models.Location]
	areasLock            *sync.RWMutex
	areas                models.LocationAreas
}

// LocationRegistry is a context-aware registry that can only be created through the factory
type LocationRegistry struct {
	*Registry[models.Location, *models.Location]

	userID    string
	areasLock *sync.RWMutex
	areas     models.LocationAreas
}

var _ registry.LocationRegistry = (*LocationRegistry)(nil)
var _ registry.LocationRegistryFactory = (*LocationRegistryFactory)(nil)

func NewLocationRegistry() *LocationRegistryFactory {
	return &LocationRegistryFactory{
		baseLocationRegistry: NewRegistry[models.Location, *models.Location](),
		areasLock:            &sync.RWMutex{},
		areas:                make(models.LocationAreas),
	}
}

// Factory methods implementing registry.LocationRegistryFactory

func (f *LocationRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.LocationRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *LocationRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.LocationRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.Location, *models.Location]{
		items:  f.baseLocationRegistry.items, // Share the data map
		lock:   f.baseLocationRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                      // Set user-specific userID
	}

	return &LocationRegistry{
		Registry:  userRegistry,
		userID:    user.ID,
		areasLock: f.areasLock,
		areas:     f.areas,
	}, nil
}

func (f *LocationRegistryFactory) CreateServiceRegistry() registry.LocationRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.Location, *models.Location]{
		items:  f.baseLocationRegistry.items, // Share the data map
		lock:   f.baseLocationRegistry.lock,  // Share the mutex pointer
		userID: "",                           // Clear userID to bypass user filtering
	}

	return &LocationRegistry{
		Registry:  serviceRegistry,
		userID:    "", // Clear userID to bypass user filtering
		areasLock: f.areasLock,
		areas:     f.areas,
	}
}

func (r *LocationRegistry) Delete(ctx context.Context, id string) error {
	_, err := r.Registry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get location")
	}

	if len(must.Must(r.GetAreas(ctx, id))) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "location has areas")
	}

	err = r.Registry.Delete(ctx, id)
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

	r.areas[locationID] = slices.DeleteFunc(r.areas[locationID], func(id string) bool {
		return id == areaID
	})

	r.areasLock.Unlock()

	return nil
}

// Enhanced methods with simplified in-memory implementations

// GetAreaCount returns the number of areas in a location (simplified)
func (r *LocationRegistry) GetAreaCount(ctx context.Context, locationID string) (int, error) {
	areas, err := r.GetAreas(ctx, locationID)
	if err != nil {
		return 0, err
	}
	return len(areas), nil
}

// GetTotalCommodityCount returns the total number of commodities across all areas in a location (simplified)
func (r *LocationRegistry) GetTotalCommodityCount(ctx context.Context, locationID string) (int, error) {
	// This is a simplified implementation that would require access to area and commodity data
	// In a real implementation, this would need to be coordinated with the area and commodity registries
	// For now, return 0 as a placeholder
	return 0, nil
}

// SearchByName searches locations by name using simple text matching (simplified)
func (r *LocationRegistry) SearchByName(ctx context.Context, query string) ([]*models.Location, error) {
	locations, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []*models.Location

	for _, location := range locations {
		if strings.Contains(strings.ToLower(location.Name), query) {
			filtered = append(filtered, location)
		}
	}

	return filtered, nil
}
