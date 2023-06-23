package registry

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type ManualRegistry interface {
	Registry[models.Manual]
}

type baseMemoryManualRegistry = MemoryRegistry[models.Manual]
type MemoryManualRegistry struct {
	*baseMemoryManualRegistry

	commodityRegistry CommodityRegistry
}

func NewMemoryManualRegistry(commodityRegistry CommodityRegistry) *MemoryManualRegistry {
	return &MemoryManualRegistry{
		baseMemoryManualRegistry: NewMemoryRegistry[models.Manual](),
		commodityRegistry:        commodityRegistry,
	}
}

func (r *MemoryManualRegistry) Create(manual models.Manual) (*models.Manual, error) {
	err := validation.Validate(&manual)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.commodityRegistry.Get(manual.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newManual, err := r.baseMemoryManualRegistry.Create(manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create manual")
	}

	r.commodityRegistry.AddManual(manual.CommodityID, newManual.ID)

	return newManual, err
}

func (r *MemoryManualRegistry) Delete(id string) error {
	manual, err := r.baseMemoryManualRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseMemoryManualRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.commodityRegistry.DeleteManual(manual.CommodityID, id)

	return nil
}
