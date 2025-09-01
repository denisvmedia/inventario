package memory

import (
	"context"
	"errors"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ManualRegistryFactory creates ManualRegistry instances with proper context
type ManualRegistryFactory struct {
	baseManualRegistry *Registry[models.Manual, *models.Manual]
	commodityRegistry  *CommodityRegistryFactory // required dependency for relationship tracking
}

// ManualRegistry is a context-aware registry that can only be created through the factory
type ManualRegistry struct {
	*Registry[models.Manual, *models.Manual]

	userID            string
	commodityRegistry *CommodityRegistry // required dependency for relationship tracking
}

var _ registry.ManualRegistry = (*ManualRegistry)(nil)
var _ registry.ManualRegistryFactory = (*ManualRegistryFactory)(nil)

func NewManualRegistry(commodityRegistry *CommodityRegistryFactory) *ManualRegistryFactory {
	return &ManualRegistryFactory{
		baseManualRegistry: NewRegistry[models.Manual, *models.Manual](),
		commodityRegistry:  commodityRegistry,
	}
}

// Factory methods implementing registry.ManualRegistryFactory

func (f *ManualRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ManualRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *ManualRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ManualRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.Manual, *models.Manual]{
		items:  f.baseManualRegistry.items, // Share the data map
		lock:   f.baseManualRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                    // Set user-specific userID
	}

	// Create user-aware commodity registry
	commodityRegistryInterface, err := f.commodityRegistry.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create user commodity registry")
	}

	// Cast to concrete type for relationship management
	commodityRegistry, ok := commodityRegistryInterface.(*CommodityRegistry)
	if !ok {
		return nil, errors.New("failed to cast commodity registry to concrete type")
	}

	return &ManualRegistry{
		Registry:          userRegistry,
		userID:            user.ID,
		commodityRegistry: commodityRegistry,
	}, nil
}

func (f *ManualRegistryFactory) CreateServiceRegistry() registry.ManualRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.Manual, *models.Manual]{
		items:  f.baseManualRegistry.items, // Share the data map
		lock:   f.baseManualRegistry.lock,  // Share the mutex pointer
		userID: "",                         // Clear userID to bypass user filtering
	}

	// Create service-aware commodity registry
	commodityRegistryInterface := f.commodityRegistry.CreateServiceRegistry()

	// Cast to concrete type for relationship management
	commodityRegistry := commodityRegistryInterface.(*CommodityRegistry)

	return &ManualRegistry{
		Registry:          serviceRegistry,
		userID:            "", // Clear userID to bypass user filtering
		commodityRegistry: commodityRegistry,
	}
}

func (r *ManualRegistry) Create(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// Use CreateWithUser to ensure user context is applied
	newManual, err := r.Registry.CreateWithUser(ctx, manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create manual")
	}

	// Add this manual to its parent commodity's manual list
	_ = r.commodityRegistry.AddManual(ctx, newManual.CommodityID, newManual.GetID())

	return newManual, nil
}

func (r *ManualRegistry) Update(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// Get the existing manual to check if CommodityID changed
	var oldCommodityID string
	if existingManual, err := r.Registry.Get(ctx, manual.GetID()); err == nil {
		oldCommodityID = existingManual.CommodityID
	}

	// Call the base registry's UpdateWithUser method to ensure user context is preserved
	updatedManual, err := r.Registry.UpdateWithUser(ctx, manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update manual")
	}

	// Handle commodity registry tracking - commodity changed
	if oldCommodityID != "" && oldCommodityID != updatedManual.CommodityID {
		// Remove from old commodity
		_ = r.commodityRegistry.DeleteManual(ctx, oldCommodityID, updatedManual.GetID())
		// Add to new commodity
		_ = r.commodityRegistry.AddManual(ctx, updatedManual.CommodityID, updatedManual.GetID())
	} else if oldCommodityID == "" {
		// This is a fallback case - add to commodity if not already tracked
		_ = r.commodityRegistry.AddManual(ctx, updatedManual.CommodityID, updatedManual.GetID())
	}

	return updatedManual, nil
}

func (r *ManualRegistry) Delete(ctx context.Context, id string) error {
	// Remove this manual from its parent commodity's manual list
	manual, err := r.Registry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get manual")
	}

	_ = r.commodityRegistry.DeleteManual(ctx, manual.CommodityID, id)

	err = r.Registry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual")
	}

	err = r.commodityRegistry.DeleteManual(ctx, manual.CommodityID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual from commodity")
	}

	return nil
}
