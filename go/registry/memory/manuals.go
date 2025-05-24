package memory

import (
	"context"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ManualRegistry = (*ManualRegistry)(nil)

type baseManualRegistry = Registry[models.Manual, *models.Manual]
type ManualRegistry struct {
	*baseManualRegistry

	commodityRegistry registry.CommodityRegistry
}

func NewManualRegistry(commodityRegistry registry.CommodityRegistry) *ManualRegistry {
	return &ManualRegistry{
		baseManualRegistry: NewRegistry[models.Manual, *models.Manual](),
		commodityRegistry:  commodityRegistry,
	}
}

func (r *ManualRegistry) Create(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	err := validation.Validate(&manual)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.commodityRegistry.Get(ctx, manual.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newManual, err := r.baseManualRegistry.Create(ctx, manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create manual")
	}

	err = r.commodityRegistry.AddManual(ctx, manual.CommodityID, newManual.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to add manual to commodity")
	}

	return newManual, nil
}

func (r *ManualRegistry) Delete(ctx context.Context, id string) error {
	manual, err := r.baseManualRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get manual")
	}

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
