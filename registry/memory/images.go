package memory

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ImageRegistry = (*ImageRegistry)(nil)

type baseImageRegistry = Registry[models.Image, *models.Image]
type ImageRegistry struct {
	*baseImageRegistry

	commodityRegistry registry.CommodityRegistry
}

func NewImageRegistry(commodityRegistry registry.CommodityRegistry) *ImageRegistry {
	return &ImageRegistry{
		baseImageRegistry: NewRegistry[models.Image, *models.Image](),
		commodityRegistry: commodityRegistry,
	}
}

func (r *ImageRegistry) Create(image models.Image) (*models.Image, error) {
	err := validation.Validate(&image)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.commodityRegistry.Get(image.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newImage, err := r.baseImageRegistry.Create(image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create image")
	}

	r.commodityRegistry.AddImage(image.CommodityID, newImage.ID)

	return newImage, err
}

func (r *ImageRegistry) Delete(id string) error {
	image, err := r.baseImageRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseImageRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.commodityRegistry.DeleteImage(image.CommodityID, id)

	return nil
}
