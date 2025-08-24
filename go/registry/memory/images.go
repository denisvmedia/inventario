package memory

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ImageRegistry = (*ImageRegistry)(nil)

type baseImageRegistry = Registry[models.Image, *models.Image]
type ImageRegistry struct {
	*baseImageRegistry

	userID            string
	commodityRegistry *CommodityRegistry // required dependency for relationship tracking
}

func NewImageRegistry(commodityRegistry *CommodityRegistry) *ImageRegistry {
	return &ImageRegistry{
		baseImageRegistry: NewRegistry[models.Image, *models.Image](),
		commodityRegistry: commodityRegistry,
	}
}

func (r *ImageRegistry) MustWithCurrentUser(ctx context.Context) registry.ImageRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *ImageRegistry) WithCurrentUser(ctx context.Context) (registry.ImageRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}
	tmp.userID = user.ID
	return &tmp, nil
}

func (r *ImageRegistry) Create(ctx context.Context, image models.Image) (*models.Image, error) {
	// Use CreateWithUser to ensure user context is applied
	newImage, err := r.baseImageRegistry.CreateWithUser(ctx, image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create image")
	}

	// Add this image to its parent commodity's image list
	_ = r.commodityRegistry.AddImage(ctx, newImage.CommodityID, newImage.GetID())

	return newImage, nil
}

func (r *ImageRegistry) Update(ctx context.Context, image models.Image) (*models.Image, error) {
	// Get the existing image to check if CommodityID changed
	var oldCommodityID string
	if existingImage, err := r.baseImageRegistry.Get(ctx, image.GetID()); err == nil {
		oldCommodityID = existingImage.CommodityID
	}

	// Call the base registry's Update method
	updatedImage, err := r.baseImageRegistry.Update(ctx, image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update image")
	}

	// Handle commodity registry tracking - commodity changed
	if oldCommodityID != "" && oldCommodityID != updatedImage.CommodityID {
		// Remove from old commodity
		_ = r.commodityRegistry.DeleteImage(ctx, oldCommodityID, updatedImage.GetID())
		// Add to new commodity
		_ = r.commodityRegistry.AddImage(ctx, updatedImage.CommodityID, updatedImage.GetID())
	} else if oldCommodityID == "" {
		// This is a fallback case - add to commodity if not already tracked
		_ = r.commodityRegistry.AddImage(ctx, updatedImage.CommodityID, updatedImage.GetID())
	}

	return updatedImage, nil
}

func (r *ImageRegistry) Delete(ctx context.Context, id string) error {
	// Remove this image from its parent commodity's image list
	image, err := r.baseImageRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get image")
	}

	_ = r.commodityRegistry.DeleteImage(ctx, image.CommodityID, id)

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
