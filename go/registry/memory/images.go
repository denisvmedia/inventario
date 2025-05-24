package memory

import (
	"context"

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

func (r *ImageRegistry) Create(ctx context.Context, image models.Image) (*models.Image, error) {
	err := validation.Validate(&image)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.commodityRegistry.Get(ctx, image.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newImage, err := r.baseImageRegistry.Create(ctx, image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create image")
	}

	err = r.commodityRegistry.AddImage(ctx, image.CommodityID, newImage.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed adding image")
	}

	return newImage, nil
}

func (r *ImageRegistry) Delete(ctx context.Context, id string) error {
	image, err := r.baseImageRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get image")
	}

	err = r.baseImageRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete image")
	}

	err = r.commodityRegistry.DeleteImage(ctx, image.CommodityID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete image from commodity")
	}

	return nil
}
