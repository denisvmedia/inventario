package registry

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type ImageRegistry interface {
	Registry[models.Image]
}

type baseMemoryImageRegistry = MemoryRegistry[models.Image]
type MemoryImageRegistry struct {
	*baseMemoryImageRegistry

	commodityRegistry CommodityRegistry
}

func NewMemoryImageRegistry(commodityRegistry CommodityRegistry) *MemoryImageRegistry {
	return &MemoryImageRegistry{
		baseMemoryImageRegistry: NewMemoryRegistry[models.Image](),
		commodityRegistry:       commodityRegistry,
	}
}

func (r *MemoryImageRegistry) Create(image models.Image) (*models.Image, error) {
	err := validation.Validate(&image)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.commodityRegistry.Get(image.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newImage, err := r.baseMemoryImageRegistry.Create(image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create image")
	}

	r.commodityRegistry.AddImage(image.CommodityID, newImage.ID)

	return newImage, err
}

func (r *MemoryImageRegistry) Delete(id string) error {
	image, err := r.baseMemoryImageRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseMemoryImageRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.commodityRegistry.DeleteImage(image.CommodityID, id)

	return nil
}
