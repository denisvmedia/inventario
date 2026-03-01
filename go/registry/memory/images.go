package memory

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ImageRegistryFactory creates ImageRegistry instances with proper context
type ImageRegistryFactory struct {
	baseImageRegistry *Registry[models.Image, *models.Image]
	commodityRegistry *CommodityRegistryFactory // required dependency for relationship tracking
}

// ImageRegistry is a context-aware registry that can only be created through the factory
type ImageRegistry baseCommodityAndUserAwareRegistry[models.Image, *models.Image]

var _ registry.ImageRegistry = (*ImageRegistry)(nil)
var _ registry.ImageRegistryFactory = (*ImageRegistryFactory)(nil)

func NewImageRegistryFactory(commodityRegistry *CommodityRegistryFactory) *ImageRegistryFactory {
	return &ImageRegistryFactory{
		baseImageRegistry: NewRegistry[models.Image, *models.Image](),
		commodityRegistry: commodityRegistry,
	}
}

// Factory methods implementing registry.ImageRegistryFactory

func (f *ImageRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ImageRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *ImageRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ImageRegistry, error) {
	result, err := createUserRegistry(ctx, func(userID string) *Registry[models.Image, *models.Image] {
		return &Registry[models.Image, *models.Image]{
			items:  f.baseImageRegistry.items, // Share the data map
			lock:   f.baseImageRegistry.lock,  // Share the mutex pointer
			userID: userID,                    // Set user-specific userID
		}
	}, f.commodityRegistry)
	if err != nil {
		return nil, err
	}
	// Convert to *ImageRegistry so callers get Create/Update/Delete with commodity relationship tracking.
	return (*ImageRegistry)(result), nil
}

func (f *ImageRegistryFactory) CreateServiceRegistry() registry.ImageRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.Image, *models.Image]{
		items:  f.baseImageRegistry.items, // Share the data map
		lock:   f.baseImageRegistry.lock,  // Share the mutex pointer
		userID: "",                        // Clear userID to bypass user filtering
	}

	// Create service-aware commodity registry
	commodityRegistryInterface := f.commodityRegistry.CreateServiceRegistry()

	// Cast to concrete type for relationship management
	commodityRegistry, ok := commodityRegistryInterface.(*CommodityRegistry)
	if !ok {
		panic("commodityRegistryInterface is not of type *CommodityRegistry")
	}

	return &ImageRegistry{
		Registry:          serviceRegistry,
		userID:            "", // Clear userID to bypass user filtering
		commodityRegistry: commodityRegistry,
	}
}

func (r *ImageRegistry) Create(ctx context.Context, image models.Image) (*models.Image, error) {
	// Use CreateWithUser to ensure user context is applied
	newImage, err := r.Registry.CreateWithUser(ctx, image)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create image", err)
	}

	// Add this image to its parent commodity's image list
	_ = r.commodityRegistry.AddImage(ctx, newImage.CommodityID, newImage.GetID())

	return newImage, nil
}

func (r *ImageRegistry) Update(ctx context.Context, image models.Image) (*models.Image, error) {
	// Get the existing image to check if CommodityID changed
	var oldCommodityID string
	if existingImage, err := r.Registry.Get(ctx, image.GetID()); err == nil {
		oldCommodityID = existingImage.CommodityID
	}

	// Call the base registry's UpdateWithUser method to ensure user context is preserved
	updatedImage, err := r.Registry.UpdateWithUser(ctx, image)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update image", err)
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
	image, err := r.Registry.Get(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get image", err)
	}

	_ = r.commodityRegistry.DeleteImage(ctx, image.CommodityID, id)

	err = r.Registry.Delete(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to delete image", err)
	}

	err = r.commodityRegistry.DeleteImage(ctx, image.CommodityID, id)
	if err != nil {
		return errxtrace.Wrap("failed to delete image from commodity", err)
	}

	return nil
}
