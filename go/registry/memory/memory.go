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
	restoreStepFactory := NewRestoreStepRegistryFactory()
	restoreOperationFactory := NewRestoreOperationRegistryFactory(restoreStepFactory)
	exportFactory := NewExportRegistryFactory(restoreOperationFactory)
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
	// Magic-link sign-in tokens — service-mode lookup resolved before any
	// user session exists (same posture as reset/verification tokens).
	fs.MagicLinkTokenRegistry = NewMagicLinkTokenRegistry()
	// OAuth identities (#1394) — service-mode lookup keyed by
	// (provider, provider_user_id) during the callback before any user
	// session exists.
	fs.OAuthIdentityRegistry = NewOAuthIdentityRegistry()
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
	// Background-worker soft-pause control (issue #1308). Global control
	// plane — no tenant scope, no RLS (mirrors SystemAdminGrantRegistry).
	fs.WorkerControlRegistry = NewWorkerControlRegistry()
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
		commodityLoanFactory,
		commodityServiceFactory,
		supplyLinkFactory,
		tagFactory,
		exportFactory,
		restoreOperationFactory,
		restoreStepFactory,
		fileFactory,
		thumbnailGenerationJobFactory,
		userConcurrencySlotFactory,
		maintenanceScheduleFactory,
		fs.MaintenanceReminderRegistry,
		fs.CurrencyMigrationRegistryFactory,
		fs.GroupNotificationPrefRegistry,
		fs.GroupMembershipRegistry,
	)
	// UserPurger (#2116): clears a single user's auth/identity rows during the
	// admin user hard-delete (and, since #2147, the user's group_invites_audit
	// references). Takes the plain singleton registries that own the shared
	// in-memory maps; all are set above, so it can be wired here.
	fs.UserPurger = NewUserPurger(
		fs.RefreshTokenRegistry,
		fs.UserMFASecretRegistry,
		fs.OAuthIdentityRegistry,
		fs.PasswordResetRegistry,
		fs.EmailVerificationRegistry,
		fs.MagicLinkTokenRegistry,
		fs.GroupMembershipRegistry,
		fs.SystemAdminGrantRegistry,
		fs.GroupInviteAuditRegistry,
	)
	// SystemStats (#843): the memory backend is dev/test only and its
	// data registries are tenant/group-scoped behind the per-request
	// context, with no cheap installation-wide roll-up. Rather than range
	// every registry's internal maps (which would couple this constructor
	// to each registry's storage shape), report zeros for the business
	// gauges. A non-nil zero-returning closure is preferred over leaving
	// the field nil so the collector's behaviour is predictable in dev:
	// the gauges publish at 0 instead of being skipped entirely. The
	// postgres backend is the real producer of business metrics.
	fs.SystemStats = func(context.Context) (registry.SystemStats, error) {
		return registry.SystemStats{}, nil
	}
	fs.PingFn = func(context.Context) error { return nil }

	// TenantPurger (#2115) reads almost every registry off the FactorySet, so
	// it MUST be wired LAST — after every other fs.* field is populated above.
	fs.TenantPurger = NewTenantPurger(fs)

	// UserContentOwnershipChecker (#2147) likewise reads several content
	// registries off the FactorySet, so it is wired after every fs.* field too.
	fs.UserContentOwnershipChecker = NewUserContentOwnershipChecker(fs)

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
