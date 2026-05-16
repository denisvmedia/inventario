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
	commodityEventFactory := NewCommodityEventRegistryFactory()
	tagFactory := NewTagRegistryFactory(commodityFactory, fileFactory)
	commodityLoanFactory := NewCommodityLoanRegistryFactory()
	commodityServiceFactory := NewCommodityServiceRegistryFactory()
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
	fs.CommodityEventRegistryFactory = commodityEventFactory
	fs.TagRegistryFactory = tagFactory
	fs.CommodityLoanRegistryFactory = commodityLoanFactory
	fs.CommodityServiceRegistryFactory = commodityServiceFactory
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
	fs.LoginEventRegistry = NewLoginEventRegistry()
	fs.UserMFASecretRegistry = NewUserMFASecretRegistry()
	fs.AuditLogRegistry = NewAuditLogRegistry()
	fs.LocationGroupRegistry = NewLocationGroupRegistry()
	membershipReg := NewGroupMembershipRegistry()
	membershipReg.SetUserRegistry(fs.UserRegistry)
	fs.GroupMembershipRegistry = membershipReg
	fs.GroupInviteRegistry = NewGroupInviteRegistry()
	fs.GroupInviteAuditRegistry = NewGroupInviteAuditRegistry()
	fs.GroupNotificationPrefRegistry = NewGroupNotificationPrefRegistry()
	fs.WarrantyReminderRegistry = NewWarrantyReminderRegistry()
	fs.StorageQuotaReminderRegistry = NewStorageQuotaReminderRegistry()
	fs.CurrencyMigrationRegistryFactory = NewCurrencyMigrationRegistryFactory()
	fs.GroupPurger = NewGroupPurger(
		locationFactory,
		areaFactory,
		commodityFactory,
		commodityEventFactory,
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
