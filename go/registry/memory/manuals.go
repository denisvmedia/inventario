package memory

import (
	"context"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ManualRegistry = (*ManualRegistry)(nil)

type baseManualRegistry = Registry[models.Manual, *models.Manual]
type ManualRegistry struct {
	*baseManualRegistry

	userID            string
	commodityRegistry *CommodityRegistry // required dependency for relationship tracking
}

func NewManualRegistry(commodityRegistry *CommodityRegistry) *ManualRegistry {
	return &ManualRegistry{
		baseManualRegistry: NewRegistry[models.Manual, *models.Manual](),
		commodityRegistry:  commodityRegistry,
	}
}

func (r *ManualRegistry) WithCurrentUser(ctx context.Context) (registry.ManualRegistry, error) {
	tmp := *r

	userID, err := appctx.RequireUserIDFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}
	tmp.userID = userID
	return &tmp, nil
}

func (r *ManualRegistry) Create(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// Use CreateWithUser to ensure user context is applied
	newManual, err := r.baseManualRegistry.CreateWithUser(ctx, manual)
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
	if existingManual, err := r.baseManualRegistry.Get(ctx, manual.GetID()); err == nil {
		oldCommodityID = existingManual.CommodityID
	}

	// Call the base registry's Update method
	updatedManual, err := r.baseManualRegistry.Update(ctx, manual)
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
	manual, err := r.baseManualRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get manual")
	}

	_ = r.commodityRegistry.DeleteManual(ctx, manual.CommodityID, id)

	err = r.baseManualRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual")
	}

	err = r.commodityRegistry.DeleteManual(ctx, manual.CommodityID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual from commodity")
	}

	return nil
}
