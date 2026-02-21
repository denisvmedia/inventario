package registry

import (
	"context"

	"github.com/denisvmedia/inventario/models"
)

// AreaRegistryFactory creates AreaRegistry instances with proper context
type AreaRegistryFactory interface {
	UserRegistryFactory[models.Area, AreaRegistry]
	ServiceRegistryFactory[models.Area, AreaRegistry]
}

// CommodityRegistryFactory creates CommodityRegistry instances with proper context
type CommodityRegistryFactory interface {
	UserRegistryFactory[models.Commodity, CommodityRegistry]
	ServiceRegistryFactory[models.Commodity, CommodityRegistry]
}

// LocationRegistryFactory creates LocationRegistry instances with proper context
type LocationRegistryFactory interface {
	UserRegistryFactory[models.Location, LocationRegistry]
	ServiceRegistryFactory[models.Location, LocationRegistry]
}

// ImageRegistryFactory creates ImageRegistry instances with proper context
type ImageRegistryFactory interface {
	UserRegistryFactory[models.Image, ImageRegistry]
	ServiceRegistryFactory[models.Image, ImageRegistry]
}

// InvoiceRegistryFactory creates InvoiceRegistry instances with proper context
type InvoiceRegistryFactory interface {
	UserRegistryFactory[models.Invoice, InvoiceRegistry]
	ServiceRegistryFactory[models.Invoice, InvoiceRegistry]
}

// ManualRegistryFactory creates ManualRegistry instances with proper context
type ManualRegistryFactory interface {
	UserRegistryFactory[models.Manual, ManualRegistry]
	ServiceRegistryFactory[models.Manual, ManualRegistry]
}

// SettingsRegistryFactory creates SettingsRegistry instances with proper context
type SettingsRegistryFactory interface {
	// CreateUserRegistry creates a new registry with user context from the provided context
	CreateUserRegistry(ctx context.Context) (SettingsRegistry, error)
	// MustCreateUserRegistry creates a new registry with user context, panics on error
	MustCreateUserRegistry(ctx context.Context) SettingsRegistry
	// CreateServiceRegistry creates a new registry with service account context
	CreateServiceRegistry() SettingsRegistry
}

// ExportRegistryFactory creates ExportRegistry instances with proper context
type ExportRegistryFactory interface {
	UserRegistryFactory[models.Export, ExportRegistry]
	ServiceRegistryFactory[models.Export, ExportRegistry]
}

// FileRegistryFactory creates FileRegistry instances with proper context
type FileRegistryFactory interface {
	UserRegistryFactory[models.FileEntity, FileRegistry]
	ServiceRegistryFactory[models.FileEntity, FileRegistry]
}

// RestoreOperationRegistryFactory creates RestoreOperationRegistry instances with proper context
type RestoreOperationRegistryFactory interface {
	UserRegistryFactory[models.RestoreOperation, RestoreOperationRegistry]
	ServiceRegistryFactory[models.RestoreOperation, RestoreOperationRegistry]
}

// RestoreStepRegistryFactory creates RestoreStepRegistry instances with proper context
type RestoreStepRegistryFactory interface {
	UserRegistryFactory[models.RestoreStep, RestoreStepRegistry]
	ServiceRegistryFactory[models.RestoreStep, RestoreStepRegistry]
}

// ThumbnailGenerationJobRegistryFactory creates ThumbnailGenerationJobRegistry instances with proper context
type ThumbnailGenerationJobRegistryFactory interface {
	UserRegistryFactory[models.ThumbnailGenerationJob, ThumbnailGenerationJobRegistry]
	ServiceRegistryFactory[models.ThumbnailGenerationJob, ThumbnailGenerationJobRegistry]
}

// UserConcurrencySlotRegistryFactory creates UserConcurrencySlotRegistry instances with proper context
type UserConcurrencySlotRegistryFactory interface {
	UserRegistryFactory[models.UserConcurrencySlot, UserConcurrencySlotRegistry]
	ServiceRegistryFactory[models.UserConcurrencySlot, UserConcurrencySlotRegistry]
}

// OperationSlotRegistryFactory creates OperationSlotRegistry instances with proper context
type OperationSlotRegistryFactory interface {
	UserRegistryFactory[models.OperationSlot, OperationSlotRegistry]
	ServiceRegistryFactory[models.OperationSlot, OperationSlotRegistry]
}

// FactorySet contains all registry factories - these create safe, context-aware registries
type FactorySet struct {
	LocationRegistryFactory               LocationRegistryFactory
	AreaRegistryFactory                   AreaRegistryFactory
	CommodityRegistryFactory              CommodityRegistryFactory
	ImageRegistryFactory                  ImageRegistryFactory
	InvoiceRegistryFactory                InvoiceRegistryFactory
	ManualRegistryFactory                 ManualRegistryFactory
	SettingsRegistryFactory               SettingsRegistryFactory
	ExportRegistryFactory                 ExportRegistryFactory
	RestoreOperationRegistryFactory       RestoreOperationRegistryFactory
	RestoreStepRegistryFactory            RestoreStepRegistryFactory
	FileRegistryFactory                   FileRegistryFactory
	ThumbnailGenerationJobRegistryFactory ThumbnailGenerationJobRegistryFactory
	UserConcurrencySlotRegistryFactory    UserConcurrencySlotRegistryFactory
	OperationSlotRegistryFactory          OperationSlotRegistryFactory
	TenantRegistry                        TenantRegistry       // TenantRegistry doesn't need factory as it's not user-aware
	UserRegistry                          UserRegistry         // UserRegistry doesn't need factory as it's not user-aware
	RefreshTokenRegistry                  RefreshTokenRegistry // RefreshTokenRegistry doesn't need factory as it's not user-aware
}

// CreateUserRegistrySet creates a complete set of user-aware registries from factories
func (fs *FactorySet) CreateUserRegistrySet(ctx context.Context) (*Set, error) {
	locationRegistry, err := fs.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	areaRegistry, err := fs.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	commodityRegistry, err := fs.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	imageRegistry, err := fs.ImageRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	invoiceRegistry, err := fs.InvoiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	manualRegistry, err := fs.ManualRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	settingsRegistry, err := fs.SettingsRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	exportRegistry, err := fs.ExportRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	fileRegistry, err := fs.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	restoreOperationRegistry, err := fs.RestoreOperationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	restoreStepRegistry, err := fs.RestoreStepRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	thumbnailGenerationJobRegistry, err := fs.ThumbnailGenerationJobRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	userConcurrencySlotRegistry, err := fs.UserConcurrencySlotRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	operationSlotRegistry, err := fs.OperationSlotRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	return &Set{
		LocationRegistry:               locationRegistry,
		AreaRegistry:                   areaRegistry,
		CommodityRegistry:              commodityRegistry,
		ImageRegistry:                  imageRegistry,
		InvoiceRegistry:                invoiceRegistry,
		ManualRegistry:                 manualRegistry,
		SettingsRegistry:               settingsRegistry,
		ExportRegistry:                 exportRegistry,
		RestoreOperationRegistry:       restoreOperationRegistry,
		RestoreStepRegistry:            restoreStepRegistry,
		FileRegistry:                   fileRegistry,
		ThumbnailGenerationJobRegistry: thumbnailGenerationJobRegistry,
		UserConcurrencySlotRegistry:    userConcurrencySlotRegistry,
		OperationSlotRegistry:          operationSlotRegistry,
		TenantRegistry:                 fs.TenantRegistry,
		UserRegistry:                   fs.UserRegistry,
		RefreshTokenRegistry:           fs.RefreshTokenRegistry,
	}, nil
}

// CreateServiceRegistrySet creates a complete set of service-aware registries from factories
func (fs *FactorySet) CreateServiceRegistrySet() *Set {
	return &Set{
		LocationRegistry:               fs.LocationRegistryFactory.CreateServiceRegistry(),
		AreaRegistry:                   fs.AreaRegistryFactory.CreateServiceRegistry(),
		CommodityRegistry:              fs.CommodityRegistryFactory.CreateServiceRegistry(),
		ImageRegistry:                  fs.ImageRegistryFactory.CreateServiceRegistry(),
		InvoiceRegistry:                fs.InvoiceRegistryFactory.CreateServiceRegistry(),
		ManualRegistry:                 fs.ManualRegistryFactory.CreateServiceRegistry(),
		SettingsRegistry:               fs.SettingsRegistryFactory.CreateServiceRegistry(),
		ExportRegistry:                 fs.ExportRegistryFactory.CreateServiceRegistry(),
		RestoreOperationRegistry:       fs.RestoreOperationRegistryFactory.CreateServiceRegistry(),
		RestoreStepRegistry:            fs.RestoreStepRegistryFactory.CreateServiceRegistry(),
		FileRegistry:                   fs.FileRegistryFactory.CreateServiceRegistry(),
		ThumbnailGenerationJobRegistry: fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry(),
		UserConcurrencySlotRegistry:    fs.UserConcurrencySlotRegistryFactory.CreateServiceRegistry(),
		OperationSlotRegistry:          fs.OperationSlotRegistryFactory.CreateServiceRegistry(),
		TenantRegistry:                 fs.TenantRegistry,
		UserRegistry:                   fs.UserRegistry,
		RefreshTokenRegistry:           fs.RefreshTokenRegistry,
	}
}
