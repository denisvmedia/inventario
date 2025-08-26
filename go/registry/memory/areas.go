package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.AreaRegistry = (*AreaRegistry)(nil)

type baseAreaRegistry = Registry[models.Area, *models.Area]

type AreaRegistry struct {
	*baseAreaRegistry

	userID           string
	commoditiesLock  sync.RWMutex
	commodities      models.AreaCommodities
	locationRegistry *LocationRegistry // required dependency for relationship tracking
}

func NewAreaRegistry(locationRegistry *LocationRegistry) *AreaRegistry {
	return &AreaRegistry{
		baseAreaRegistry: NewRegistry[models.Area, *models.Area](),
		commodities:      make(models.AreaCommodities),
		locationRegistry: locationRegistry,
	}
}

func (r *AreaRegistry) MustWithCurrentUser(ctx context.Context) registry.AreaRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *AreaRegistry) WithCurrentUser(ctx context.Context) (registry.AreaRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}

	// Create a new registry with the same data but different userID
	tmp := &AreaRegistry{
		baseAreaRegistry: r.baseAreaRegistry,
		userID:           user.ID,
		commodities:      r.commodities,
		locationRegistry: r.locationRegistry,
	}

	// Set the userID on the base registry
	tmp.baseAreaRegistry.userID = user.ID

	return tmp, nil
}

func (r *AreaRegistry) WithServiceAccount() registry.AreaRegistry {
	// For memory registries, service account access is the same as regular access
	// since memory registries don't enforce RLS restrictions
	return r
}

func (r *AreaRegistry) Create(ctx context.Context, area models.Area) (*models.Area, error) {
	// Use CreateWithUser to ensure user context is applied
	newArea, err := r.baseAreaRegistry.CreateWithUser(ctx, area)
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
	if existingArea, err := r.baseAreaRegistry.Get(ctx, area.GetID()); err == nil {
		oldLocationID = existingArea.LocationID
	}

	// Call the base registry's Update method
	updatedArea, err := r.baseAreaRegistry.Update(ctx, area)
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
	area, err := r.baseAreaRegistry.Get(ctx, id)
	if err == nil {
		_ = r.locationRegistry.DeleteArea(ctx, area.LocationID, id)
	}

	err = r.baseAreaRegistry.Delete(ctx, id)
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
	for i, foundCommodityID := range r.commodities[areaID] {
		if foundCommodityID == commodityID {
			r.commodities[areaID] = append(r.commodities[areaID][:i], r.commodities[areaID][i+1:]...)
			break
		}
	}
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
