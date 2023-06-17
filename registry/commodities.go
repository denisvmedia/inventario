package registry

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type CommodityRegistry = Registry[models.Commodity]

type baseMemoryCommodityRegistry = MemoryRegistry[models.Commodity]
type MemoryCommodityRegistry struct {
	*baseMemoryCommodityRegistry

	areaRegistry AreaRegistry
}

func NewMemoryCommodityRegistry(areaRegistry AreaRegistry) *MemoryCommodityRegistry {
	return &MemoryCommodityRegistry{
		baseMemoryCommodityRegistry: NewMemoryRegistry[models.Commodity](),
		areaRegistry:                areaRegistry,
	}
}

func (r *MemoryCommodityRegistry) Create(commodity models.Commodity) (*models.Commodity, error) {
	err := validation.Validate(commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.areaRegistry.Get(commodity.AreaID)
	if err != nil {
		return nil, errkit.Wrap(err, "area not found")
	}

	newCommodity, err := r.baseMemoryCommodityRegistry.Create(commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create commodity")
	}

	r.areaRegistry.AddCommodity(commodity.AreaID, newCommodity.ID)

	return newCommodity, err
}

func (r *MemoryCommodityRegistry) Delete(id string) error {
	commodity, err := r.baseMemoryCommodityRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseMemoryCommodityRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.areaRegistry.DeleteCommodity(commodity.AreaID, id)

	return nil
}
