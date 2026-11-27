package memory

import (
	"context"

	"go.5x5.cz/inventario/registry"
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
	savedViewFactory := NewSavedViewRegistryFactory()
	maintenanceScheduleFactory := NewMaintenanceScheduleRegistryFactory()
	commodityReminderFactory := NewCommodityReminderRegistryFactory()
	commodityDraftFactory := NewCommodityDraftRegistryFactory(commodityFactory, fileFactory)
	intakeFactory := NewIntakeRegistryFactory(fileFactory, commodityDraftFactory)
	intakeDocumentFactory := NewIntakeDocumentRegistryFactory()
	commodityDraftFactory.SetIntakeDocumentFactory(intakeDocumentFactory)
	sellerFactory := NewSellerRegistryFactory(commodityFactory)
	itemTypeFactory := NewItemTypeRegistryFactory(commodityFactory)
	customFieldFactory := NewCustomFieldRegistryFactory(commodityFactory, itemTypeFactory)
	restoreStepFactory := NewRestoreStepRegistryFactory()
	restoreOperationFactory := NewRestoreOperationRegistryFactory(restoreStepFactory)
	exportFactory := NewExportRegistryFactory(restoreOperationFactory)
	thumbnailGenerationJobFactory := NewThumbnailGenerationJobRegistryFactory()
	userConcurrencySlotFactory := NewUserConcurrencySlotRegistryFactory()
	operationSlotFactory := NewOperationSlotRegistryFactory()

	// Wire the sibling registries the orphan-file GC's candidate scan probes
	// for existence (#2237). Postgres does this with a SQL anti-join; the
	// memory backend has no planner, so the file registry has to hold the
	// three linked-entity factories. Deliberately a post-construction setter:
	// commodityFactory depends on areaFactory which depends on
	// locationFactory, and fileFactory is constructed before all of them.
	fileFactory.SetLinkedEntityFactories(commodityFactory, areaFactory, locationFactory)

	// The group-wide reads of loans, service records and maintenance
	// schedules leave out the rows of trashed items; postgres does it with a
	// join on commodities.
	commodityLoanFactory.SetCommodityFactory(commodityFactory)
	commodityServiceFactory.SetCommodityFactory(commodityFactory)
	maintenanceScheduleFactory.SetCommodityFactory(commodityFactory)

	// A tag rename rewrites the saved views that name the tag under the
	// rename's mutex.
	tagFactory.SetSavedViewFactory(savedViewFactory)

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
	fs.SavedViewRegistryFactory = savedViewFactory
	fs.MaintenanceScheduleRegistryFactory = maintenanceScheduleFactory
	fs.CommodityReminderRegistryFactory = commodityReminderFactory
	fs.CommodityDraftRegistryFactory = commodityDraftFactory
	fs.IntakeRegistryFactory = intakeFactory
	fs.IntakeDocumentRegistryFactory = intakeDocumentFactory
	fs.SellerRegistryFactory = sellerFactory
	fs.ItemTypeRegistryFactory = itemTypeFactory
	fs.CustomFieldRegistryFactory = customFieldFactory
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
	apiTokenReg := NewAPITokenRegistry()
	fs.APITokenRegistry = apiTokenReg
	personalDataExportReg := NewPersonalDataExportRegistry()
	fs.PersonalDataExportRegistry = personalDataExportReg
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
	itemTypeFactory.SetLocationGroupRegistry(groupReg)
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
	fs.WeeklyDigestSendRegistry = NewWeeklyDigestSendRegistry()
	fs.MaintenanceReminderRegistry = NewMaintenanceReminderRegistry()
	fs.CommodityReminderSendRegistry = NewCommodityReminderSendRegistry()
	currencyMigrationFactory := NewCurrencyMigrationRegistryFactory()
	currencyMigrationFactory.SetLocationGroupRegistry(groupReg)
	fs.CurrencyMigrationRegistryFactory = currencyMigrationFactory
	// Failed jobs (#1307) read the job rows of these stores directly.
	fs.FailedJobRegistry = NewFailedJobRegistry(FailedJobStores{
		Exports:             exportFactory,
		Restores:            restoreOperationFactory,
		Thumbnails:          thumbnailGenerationJobFactory,
		Files:               fileFactory,
		Intake:              intakeFactory,
		CurrencyMigrations:  currencyMigrationFactory,
		PersonalDataExports: personalDataExportReg,
		Users:               userReg,
	})
	fs.CommodityScanAuditRegistry = NewCommodityScanAuditRegistry()
	groupPurger := NewGroupPurger(
		locationFactory,
		areaFactory,
		commodityFactory,
		commodityEventFactory,
		commodityLoanFactory,
		commodityServiceFactory,
		supplyLinkFactory,
		savedViewFactory,
		tagFactory,
		sellerFactory,
		exportFactory,
		restoreOperationFactory,
		restoreStepFactory,
		fileFactory,
		thumbnailGenerationJobFactory,
		userConcurrencySlotFactory,
		maintenanceScheduleFactory,
		fs.MaintenanceReminderRegistry,
		commodityReminderFactory,
		fs.CommodityReminderSendRegistry,
		commodityDraftFactory,
		intakeFactory,
		fs.CurrencyMigrationRegistryFactory,
		fs.GroupNotificationPrefRegistry,
		fs.GroupMembershipRegistry,
	)
	groupPurger.SetItemTypeFactory(itemTypeFactory)
	groupPurger.SetIntakeDocumentFactory(intakeDocumentFactory)
	groupPurger.SetCustomFieldFactory(customFieldFactory)
	fs.GroupPurger = groupPurger
	// UserPurger (#2116): clears a single user's auth/identity rows during the
	// admin user hard-delete (and, since #2147, the user's group_invites_audit
	// references). Takes the plain singleton registries that own the shared
	// in-memory maps; all are set above, so it can be wired here.
	userPurger := NewUserPurger(
		fs.RefreshTokenRegistry,
		apiTokenReg,
		fs.UserMFASecretRegistry,
		fs.OAuthIdentityRegistry,
		fs.PasswordResetRegistry,
		fs.EmailVerificationRegistry,
		fs.MagicLinkTokenRegistry,
		fs.GroupMembershipRegistry,
		fs.SystemAdminGrantRegistry,
		fs.GroupInviteAuditRegistry,
		commodityReminderFactory,
		fs.CommodityReminderSendRegistry,
		commodityDraftFactory,
		intakeFactory,
		fileFactory,
		savedViewFactory,
	)
	userPurger.SetItemTypeFactory(itemTypeFactory)
	userPurger.SetIntakeDocumentFactory(intakeDocumentFactory)
	userPurger.SetCustomFieldFactory(customFieldFactory)
	userPurger.SetPersonalDataExportRegistry(personalDataExportReg)
	fs.UserPurger = userPurger
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

	// UserTenantMover (#2179) counts with the same stores, so it is wired
	// after every fs.* field too.
	fs.UserTenantMover = NewUserTenantMover(fs, userReg, membershipReg, userPurger)

	// The group rows the user purge hands over live in stores read off the
	// FactorySet, so they are wired after every fs.* field too.
	userPurger.setGroupRowStores(groupRowStores(fs))
	if invites, ok := fs.GroupInviteRegistry.(*GroupInviteRegistry); ok {
		userPurger.setGroupInviteRegistry(invites)
	}

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
