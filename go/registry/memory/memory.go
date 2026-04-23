package memory

import (
	"context"

	"github.com/denisvmedia/inventario/registry"
)

const Name = "memory"

func Register() (cleanup func() error) {
	newFn, cleanup := NewMemoryRegistrySet()
	registry.Register(Name, newFn)
	return cleanup
}

func NewFactorySet() *registry.FactorySet {
	// Create factory instances that will create context-aware registries
	locationFactory := NewLocationRegistryFactory()
	areaFactory := NewAreaRegistryFactory(locationFactory)
	settingsFactory := NewSettingsRegistryFactory()
	fileFactory := NewFileRegistryFactory()
	commodityFactory := NewCommodityRegistryFactory(areaFactory)
	imageFactory := NewImageRegistryFactory(commodityFactory)
	invoiceFactory := NewInvoiceRegistryFactory(commodityFactory)
	manualFactory := NewManualRegistryFactory(commodityFactory)
	exportFactory := NewExportRegistryFactory()
	restoreStepFactory := NewRestoreStepRegistryFactory()
	restoreOperationFactory := NewRestoreOperationRegistryFactory(restoreStepFactory)
	thumbnailGenerationJobFactory := NewThumbnailGenerationJobRegistryFactory()
	userConcurrencySlotFactory := NewUserConcurrencySlotRegistryFactory()
	operationSlotFactory := NewOperationSlotRegistryFactory()

	fs := &registry.FactorySet{}
	fs.LocationRegistryFactory = locationFactory
	fs.AreaRegistryFactory = areaFactory
	fs.SettingsRegistryFactory = settingsFactory
	fs.FileRegistryFactory = fileFactory
	fs.CommodityRegistryFactory = commodityFactory
	fs.ImageRegistryFactory = imageFactory
	fs.InvoiceRegistryFactory = invoiceFactory
	fs.ManualRegistryFactory = manualFactory
	fs.ExportRegistryFactory = exportFactory
	fs.RestoreStepRegistryFactory = restoreStepFactory
	fs.RestoreOperationRegistryFactory = restoreOperationFactory
	fs.ThumbnailGenerationJobRegistryFactory = thumbnailGenerationJobFactory
	fs.UserConcurrencySlotRegistryFactory = userConcurrencySlotFactory
	fs.OperationSlotRegistryFactory = operationSlotFactory
	fs.TenantRegistry = NewTenantRegistry()
	fs.EmailVerificationRegistry = NewEmailVerificationRegistry()
	fs.PasswordResetRegistry = NewPasswordResetRegistry()
	fs.UserRegistry = NewUserRegistry()
	fs.RefreshTokenRegistry = NewRefreshTokenRegistry()
	fs.AuditLogRegistry = NewAuditLogRegistry()
	fs.LocationGroupRegistry = NewLocationGroupRegistry()
	fs.GroupMembershipRegistry = NewGroupMembershipRegistry()
	fs.GroupInviteRegistry = NewGroupInviteRegistry()
	fs.GroupInviteAuditRegistry = NewGroupInviteAuditRegistry()
	fs.GroupPurger = NewGroupPurger(
		locationFactory,
		areaFactory,
		commodityFactory,
		imageFactory,
		invoiceFactory,
		manualFactory,
		exportFactory,
		restoreOperationFactory,
		restoreStepFactory,
		fileFactory,
		fs.GroupMembershipRegistry,
	)
	fs.PingFn = func(context.Context) error { return nil }

	return fs
}

func NewMemoryRegistrySet() (func(registry.Config) (*registry.FactorySet, error), func() error) {
	newFn := func(_ registry.Config) (*registry.FactorySet, error) {
		factorySet := NewFactorySet()
		return factorySet, nil
	}

	doCleanup := func() error {
		// Memory registry doesn't need cleanup
		return nil
	}

	return newFn, doCleanup
}
