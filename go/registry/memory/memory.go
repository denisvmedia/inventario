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
	supplyLinkFactory := NewSupplyLinkRegistryFactory()
	maintenanceScheduleFactory := NewMaintenanceScheduleRegistryFactory()
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
	fs.SupplyLinkRegistryFactory = supplyLinkFactory
	fs.MaintenanceScheduleRegistryFactory = maintenanceScheduleFactory
	fs.ExportRegistryFactory = exportFactory
	fs.RestoreStepRegistryFactory = restoreStepFactory
	fs.RestoreOperationRegistryFactory = restoreOperationFactory
	fs.ThumbnailGenerationJobRegistryFactory = thumbnailGenerationJobFactory
	fs.UserConcurrencySlotRegistryFactory = userConcurrencySlotFactory
	fs.OperationSlotRegistryFactory = operationSlotFactory
	tenantReg := NewTenantRegistry()
	fs.TenantRegistry = tenantReg
	fs.EmailVerificationRegistry = NewEmailVerificationRegistry()
	fs.PasswordResetRegistry = NewPasswordResetRegistry()
	userReg := NewUserRegistry()
	fs.UserRegistry = userReg
	fs.RefreshTokenRegistry = NewRefreshTokenRegistry()
	fs.LoginEventRegistry = NewLoginEventRegistry()
	fs.UserMFASecretRegistry = NewUserMFASecretRegistry()
	fs.AuditLogRegistry = NewAuditLogRegistry()
	// Back-office identities (issue #1785) — platform-operator users
	// that live OUTSIDE the tenant model. Wired on FactorySet only;
	// not part of the per-request Set since back-office identities are
	// cross-cutting infra, not user-aware data.
	fs.BackofficeUserRegistry = NewBackofficeUserRegistry()
	// Back-office refresh tokens (issue #1785, Phase 2). Separate store
	// from RefreshTokenRegistry — the FK is backoffice_user_id, no
	// tenant_id, so the two identity universes cannot cross even if a
	// hash collided.
	fs.BackofficeRefreshTokenRegistry = NewBackofficeRefreshTokenRegistry()
	// Platform-admin grant store (issue #1784) — orthogonal to the
	// back-office identity table. system_admin_grants holds *which
	// tenant users* hold platform-wide admin privilege; it has no
	// tenant scope (mirrors AuditLogRegistry).
	fs.SystemAdminGrantRegistry = NewSystemAdminGrantRegistry()
	// Back-office MFA secrets (issue #1785, Phase 4). One row per
	// back-office user; the operator CLI mints, regenerates, and wipes
	// rows. No RLS / tenant scoping — same reasoning as the rest of the
	// back-office infrastructure tables.
	fs.BackofficeUserMFASecretRegistry = NewBackofficeUserMFASecretRegistry()
	groupReg := NewLocationGroupRegistry()
	fs.LocationGroupRegistry = groupReg
	membershipReg := NewGroupMembershipRegistry()
	membershipReg.SetUserRegistry(fs.UserRegistry)
	fs.GroupMembershipRegistry = membershipReg
	// Wire the admin-listing cross-table dependencies (#1746, #1748).
	// Counts on the /api/v1/admin/tenants, /api/v1/admin/users and
	// /api/v1/admin/groups surfaces depend on these linkages; tests that
	// construct the registries directly without going through
	// NewFactorySet can leave them nil and the counts degrade to zero
	// (mirrors postgres "empty join result" semantics).
	tenantReg.SetCountRegistries(fs.UserRegistry, fs.LocationGroupRegistry)
	userReg.SetAdminListingRegistries(fs.RefreshTokenRegistry, fs.GroupMembershipRegistry)
	groupReg.SetAdminListingRegistries(fs.GroupMembershipRegistry, fs.TenantRegistry)
	fs.GroupInviteRegistry = NewGroupInviteRegistry()
	fs.GroupInviteAuditRegistry = NewGroupInviteAuditRegistry()
	fs.GroupNotificationPrefRegistry = NewGroupNotificationPrefRegistry()
	fs.WarrantyReminderRegistry = NewWarrantyReminderRegistry()
	fs.StorageQuotaReminderRegistry = NewStorageQuotaReminderRegistry()
	fs.MaintenanceReminderRegistry = NewMaintenanceReminderRegistry()
	fs.CurrencyMigrationRegistryFactory = NewCurrencyMigrationRegistryFactory()
	fs.CommodityScanAuditRegistry = NewCommodityScanAuditRegistry()
	fs.GroupPurger = NewGroupPurger(
		locationFactory,
		areaFactory,
		commodityFactory,
		commodityEventFactory,
		exportFactory,
		restoreOperationFactory,
		restoreStepFactory,
		fileFactory,
		maintenanceScheduleFactory,
		fs.MaintenanceReminderRegistry,
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
