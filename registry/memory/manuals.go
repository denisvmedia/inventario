package memory

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type baseManualRegistry = Registry[models.Manual]
type ManualRegistry struct {
	*baseManualRegistry

	commodityRegistry registry.CommodityRegistry
}

func NewManualRegistry(commodityRegistry registry.CommodityRegistry) *ManualRegistry {
	return &ManualRegistry{
		baseManualRegistry: NewRegistry[models.Manual](),
		commodityRegistry:  commodityRegistry,
	}
}

func (r *ManualRegistry) Create(manual models.Manual) (*models.Manual, error) {
	err := validation.Validate(&manual)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.commodityRegistry.Get(manual.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newManual, err := r.baseManualRegistry.Create(manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create manual")
	}

	r.commodityRegistry.AddManual(manual.CommodityID, newManual.ID)

	return newManual, err
}

func (r *ManualRegistry) Delete(id string) error {
	manual, err := r.baseManualRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseManualRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.commodityRegistry.DeleteManual(manual.CommodityID, id)

	return nil
}
