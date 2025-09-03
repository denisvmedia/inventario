package memory

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// AreaRegistryFactory creates AreaRegistry instances with proper context
type AreaRegistryFactory struct {
	baseAreaRegistry *Registry[models.Area, *models.Area]
	commoditiesLock  *sync.RWMutex
	commodities      models.AreaCommodities
	locationRegistry *LocationRegistryFactory // required dependency for relationship tracking
}

// AreaRegistry is a context-aware registry that can only be created through the factory
type AreaRegistry struct {
	*Registry[models.Area, *models.Area]

	userID           string
	commoditiesLock  *sync.RWMutex
	commodities      models.AreaCommodities
	locationRegistry *LocationRegistry // required dependency for relationship tracking
}

var _ registry.AreaRegistry = (*AreaRegistry)(nil)
var _ registry.AreaRegistryFactory = (*AreaRegistryFactory)(nil)

func NewAreaRegistryFactory(locationRegistry *LocationRegistryFactory) *AreaRegistryFactory {
	return &AreaRegistryFactory{
		baseAreaRegistry: NewRegistry[models.Area, *models.Area](),
		commoditiesLock:  &sync.RWMutex{},
		commodities:      make(models.AreaCommodities),
		locationRegistry: locationRegistry,
	}
}

// Factory methods implementing registry.AreaRegistryFactory

func (f *AreaRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.AreaRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *AreaRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.AreaRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.Area, *models.Area]{
		items:  f.baseAreaRegistry.items, // Share the data map
		lock:   f.baseAreaRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                  // Set user-specific userID
	}

	// Create user-aware location registry
	locationRegistryInterface, err := f.locationRegistry.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create user location registry")
	}

	// Cast to concrete type for relationship management
	locationRegistry, ok := locationRegistryInterface.(*LocationRegistry)
	if !ok {
		return nil, errors.New("failed to cast location registry to concrete type")
	}

	return &AreaRegistry{
		Registry:         userRegistry,
		userID:           user.ID,
		commoditiesLock:  f.commoditiesLock,
		commodities:      f.commodities,
		locationRegistry: locationRegistry,
	}, nil
}

func (f *AreaRegistryFactory) CreateServiceRegistry() registry.AreaRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.Area, *models.Area]{
		items:  f.baseAreaRegistry.items, // Share the data map
		lock:   f.baseAreaRegistry.lock,  // Share the mutex pointer
		userID: "",                       // Clear userID to bypass user filtering
	}

	// Create service-aware location registry
	locationRegistryInterface := f.locationRegistry.CreateServiceRegistry()

	// Cast to concrete type for relationship management
	locationRegistry, ok := locationRegistryInterface.(*LocationRegistry)
	if !ok {
		panic("locationRegistryInterface is not of type *LocationRegistry")
	}

	return &AreaRegistry{
		Registry:         serviceRegistry,
		userID:           "", // Clear userID to bypass user filtering
		commoditiesLock:  f.commoditiesLock,
		commodities:      f.commodities,
		locationRegistry: locationRegistry,
	}
}

func (r *AreaRegistry) Create(ctx context.Context, area models.Area) (*models.Area, error) {
	// Use CreateWithUser to ensure user context is applied
	newArea, err := r.Registry.CreateWithUser(ctx, area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create area")
	}

	// Add this area to its parent location's area list
	_ = r.locationRegistry.AddArea(ctx, newArea.LocationID, newArea.GetID())

	return newArea, nil
}

func (r *AreaRegistry) Update(ctx context.Context, area models.Area) (*models.Area, error) {
	// Get the existing area to check if LocationID changed
	var oldLocationID string
	if existingArea, err := r.Registry.Get(ctx, area.GetID()); err == nil {
		oldLocationID = existingArea.LocationID
	}

	// Call the base registry's UpdateWithUser method to ensure user context is preserved
	updatedArea, err := r.Registry.UpdateWithUser(ctx, area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update area")
	}

	// Handle location registry tracking - location changed
	if oldLocationID != "" && oldLocationID != updatedArea.LocationID {
		// Remove from old location
		_ = r.locationRegistry.DeleteArea(ctx, oldLocationID, updatedArea.GetID())
		// Add to new location
		_ = r.locationRegistry.AddArea(ctx, updatedArea.LocationID, updatedArea.GetID())
	} else if oldLocationID == "" {
		// This is a fallback case - add to location if not already tracked
		_ = r.locationRegistry.AddArea(ctx, updatedArea.LocationID, updatedArea.GetID())
	}

	return updatedArea, nil
}

func (r *AreaRegistry) Delete(ctx context.Context, id string) error {
	// Keep the constraint: refuse to delete if area still has commodities
	if len(must.Must(r.GetCommodities(ctx, id))) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "area has commodities")
	}

	// Remove this area from its parent location's area list
	area, err := r.Registry.Get(ctx, id)
	if err == nil {
		_ = r.locationRegistry.DeleteArea(ctx, area.LocationID, id)
	}

	err = r.Registry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area")
	}

	// Clean up the area's commodity list when the area is successfully deleted
	r.commoditiesLock.Lock()
	delete(r.commodities, id)
	r.commoditiesLock.Unlock()

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

	// For memory registry, return the list as-is since we don't have access to commodity registry
	// The EntityService should handle relationship cleanup
	return commodities, nil
}

func (r *AreaRegistry) DeleteCommodity(_ context.Context, areaID, commodityID string) error {
	r.commoditiesLock.Lock()

	r.commodities[areaID] = slices.DeleteFunc(r.commodities[areaID], func(id string) bool {
		return id == commodityID
	})

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
