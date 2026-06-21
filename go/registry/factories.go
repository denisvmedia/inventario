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

// CommodityEventRegistryFactory creates CommodityEventRegistry instances with proper context.
type CommodityEventRegistryFactory interface {
	UserRegistryFactory[models.CommodityEvent, CommodityEventRegistry]
	ServiceRegistryFactory[models.CommodityEvent, CommodityEventRegistry]
}

// LocationRegistryFactory creates LocationRegistry instances with proper context
type LocationRegistryFactory interface {
	UserRegistryFactory[models.Location, LocationRegistry]
	ServiceRegistryFactory[models.Location, LocationRegistry]
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

// TagRegistryFactory creates TagRegistry instances with proper context
type TagRegistryFactory interface {
	UserRegistryFactory[models.Tag, TagRegistry]
	ServiceRegistryFactory[models.Tag, TagRegistry]
}

// CommodityLoanRegistryFactory creates CommodityLoanRegistry instances with proper context
type CommodityLoanRegistryFactory interface {
	UserRegistryFactory[models.CommodityLoan, CommodityLoanRegistry]
	ServiceRegistryFactory[models.CommodityLoan, CommodityLoanRegistry]
}

// CommodityServiceRegistryFactory creates CommodityServiceRegistry instances with proper context
type CommodityServiceRegistryFactory interface {
	UserRegistryFactory[models.CommodityService, CommodityServiceRegistry]
	ServiceRegistryFactory[models.CommodityService, CommodityServiceRegistry]
}

// SupplyLinkRegistryFactory creates SupplyLinkRegistry instances with proper context (#1369).
type SupplyLinkRegistryFactory interface {
	UserRegistryFactory[models.SupplyLink, SupplyLinkRegistry]
	ServiceRegistryFactory[models.SupplyLink, SupplyLinkRegistry]
}

// MaintenanceScheduleRegistryFactory creates MaintenanceScheduleRegistry
// instances with proper context (#1368).
type MaintenanceScheduleRegistryFactory interface {
	UserRegistryFactory[models.MaintenanceSchedule, MaintenanceScheduleRegistry]
	ServiceRegistryFactory[models.MaintenanceSchedule, MaintenanceScheduleRegistry]
}

// RestoreOperationRegistryFactory creates RestoreOperationRegistry instances with proper context
type RestoreOperationRegistryFactory interface {
	UserRegistryFactory[models.RestoreOperation, RestoreOperationRegistry]
	ServiceRegistryFactory[models.RestoreOperation, RestoreOperationRegistry]
}

// CurrencyMigrationRegistryFactory creates CurrencyMigrationRegistry
// instances with proper context. The user registry is what the apiserver
// uses for preview / start / list / get; the service registry is what
// the worker uses for ClaimNextPending / SweepStuckRunning / WriteAuditRow.
//
// SetHMACKey overrides the preview-token signing key used by every
// registry the factory will subsequently produce. The bootstrap layer
// calls this once at startup with the operator-supplied key (config:
// CurrencyMigrationHMACKey) so tokens are verifiable across replicas
// and survive process restarts. Calling with an empty slice is a no-op
// (preserves the random per-process key).
type CurrencyMigrationRegistryFactory interface {
	UserRegistryFactory[models.CurrencyMigration, CurrencyMigrationRegistry]
	ServiceRegistryFactory[models.CurrencyMigration, CurrencyMigrationRegistry]
	SetHMACKey(key []byte)
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

// SystemStats is a point-in-time snapshot of installation-wide entity
// counts and storage usage. It is the registry-layer mirror of
// metrics.BusinessStats: defining it here keeps the registry package
// free of any dependency on internal/metrics (which must stay a leaf —
// it must NOT import registry). The backend constructor populates the
// SystemStatsFunc; the bootstrap layer adapts SystemStats →
// metrics.BusinessStats field-by-field when wiring the collector.
type SystemStats struct {
	Tenants        int64
	Users          int64
	LocationGroups int64
	Locations      int64
	Areas          int64
	Commodities    int64
	Files          int64

	StorageImages    int64
	StorageDocuments int64
	StorageOther     int64
	StorageExports   int64
}

// SystemStatsFunc returns a fresh installation-wide SystemStats
// snapshot. Implementations bypass tenant scoping (postgres runs the
// reads under the background-worker role) to report totals across every
// tenant and group — it is NOT user-aware and must never be invoked
// from an HTTP request path.
type SystemStatsFunc func(ctx context.Context) (SystemStats, error)

// FactorySet contains all registry factories - these create safe, context-aware registries
type FactorySet struct {
	LocationRegistryFactory               LocationRegistryFactory
	AreaRegistryFactory                   AreaRegistryFactory
	CommodityRegistryFactory              CommodityRegistryFactory
	CommodityEventRegistryFactory         CommodityEventRegistryFactory
	SettingsRegistryFactory               SettingsRegistryFactory
	ExportRegistryFactory                 ExportRegistryFactory
	RestoreOperationRegistryFactory       RestoreOperationRegistryFactory
	RestoreStepRegistryFactory            RestoreStepRegistryFactory
	FileRegistryFactory                   FileRegistryFactory
	TagRegistryFactory                    TagRegistryFactory
	CommodityLoanRegistryFactory          CommodityLoanRegistryFactory
	CommodityServiceRegistryFactory       CommodityServiceRegistryFactory
	SupplyLinkRegistryFactory             SupplyLinkRegistryFactory
	MaintenanceScheduleRegistryFactory    MaintenanceScheduleRegistryFactory
	ThumbnailGenerationJobRegistryFactory ThumbnailGenerationJobRegistryFactory
	UserConcurrencySlotRegistryFactory    UserConcurrencySlotRegistryFactory
	OperationSlotRegistryFactory          OperationSlotRegistryFactory
	PingFn                                func(context.Context) error   // Optional health check hook for backing storage.
	TenantRegistry                        TenantRegistry                // TenantRegistry doesn't need factory as it's not user-aware
	UserRegistry                          UserRegistry                  // UserRegistry doesn't need factory as it's not user-aware
	RefreshTokenRegistry                  RefreshTokenRegistry          // RefreshTokenRegistry doesn't need factory as it's not user-aware
	LoginEventRegistry                    LoginEventRegistry            // LoginEventRegistry runs under the background-worker role (write path) + app-level user_id filter (read path)
	UserMFASecretRegistry                 UserMFASecretRegistry         // Per-user TOTP secrets (#1645); service-mode (called pre-RLS in login)
	AuditLogRegistry                      AuditLogRegistry              // AuditLogRegistry doesn't need factory as it's not user-aware
	EmailVerificationRegistry             EmailVerificationRegistry     // EmailVerificationRegistry doesn't need factory as it's not user-aware
	PasswordResetRegistry                 PasswordResetRegistry         // PasswordResetRegistry doesn't need factory as it's not user-aware
	MagicLinkTokenRegistry                MagicLinkTokenRegistry        // MagicLinkTokenRegistry doesn't need factory as it's not user-aware (resolved pre-session like reset/verification)
	LocationGroupRegistry                 LocationGroupRegistry         // LocationGroupRegistry is tenant-scoped, not user-aware
	GroupMembershipRegistry               GroupMembershipRegistry       // GroupMembershipRegistry is tenant-scoped, not user-aware
	GroupInviteRegistry                   GroupInviteRegistry           // GroupInviteRegistry is tenant-scoped, not user-aware
	GroupInviteAuditRegistry              GroupInviteAuditRegistry      // GroupInviteAuditRegistry is tenant-scoped, not user-aware
	GroupNotificationPrefRegistry         GroupNotificationPrefRegistry // Per-group notification opt-outs (#1648); tenant-scoped, user-filtered in application logic
	GroupPurger                           GroupPurger                   // GroupPurger hard-deletes group-scoped data during purge ticks
	TenantPurger                          TenantPurger                  // TenantPurger hard-deletes every tenant-scoped dependent row during admin tenant hard-delete (#2115)
	UserPurger                            UserPurger                    // UserPurger hard-deletes a user's auth/identity rows during admin user hard-delete (#2116)
	WarrantyReminderRegistry              WarrantyReminderRegistry      // WarrantyReminderRegistry is the worker idempotency store; service-mode only
	StorageQuotaReminderRegistry          StorageQuotaReminderRegistry  // StorageQuotaReminderRegistry is the storage quota warning worker idempotency store; service-mode only (#1585)
	MaintenanceReminderRegistry           MaintenanceReminderRegistry   // MaintenanceReminderRegistry is the maintenance reminder worker idempotency store; service-mode only (#1368)
	CurrencyMigrationRegistryFactory      CurrencyMigrationRegistryFactory
	CommodityScanAuditRegistry            CommodityScanAuditRegistry // AI vision scan audit log (#1720); service-mode (writes audit rows even when the calling RLS context has been cancelled)
	// BackofficeUserRegistry persists platform-operator identities
	// (issue #1785). Lives on FactorySet only — back-office identities
	// are NOT part of the per-request *Set (they're cross-cutting infra,
	// not user-aware data), so the bootstrap CLI accesses them directly
	// via factorySet.BackofficeUserRegistry. Phase 2 (login flow) and
	// Phase 3 (admin surface) will add HTTP-side wiring on top.
	BackofficeUserRegistry BackofficeUserRegistry

	// BackofficeRefreshTokenRegistry persists long-lived refresh tokens
	// for the back-office auth plane (issue #1785, Phase 2). Lives on
	// FactorySet only for the same reason as BackofficeUserRegistry —
	// back-office identity infra is cross-cutting, not user-aware. The
	// HTTP login/refresh/logout handlers (Phase 2) reach in via
	// factorySet.BackofficeRefreshTokenRegistry.
	BackofficeRefreshTokenRegistry BackofficeRefreshTokenRegistry

	// SystemAdminGrantRegistry holds platform-admin grants (issue #1784).
	// Lives on FactorySet only — not tenant-scoped, not user-aware
	// (same posture as AuditLogRegistry).
	SystemAdminGrantRegistry SystemAdminGrantRegistry

	// BackofficeUserMFASecretRegistry persists per-back-office-user TOTP
	// credentials (issue #1785, Phase 4). Mirrors UserMFASecretRegistry
	// for the back-office plane: one row per back-office user, no RLS,
	// no tenant_id. The Phase 4 login handler reads it to decide whether
	// to issue a MFA challenge; the operator CLI mints / wipes rows.
	BackofficeUserMFASecretRegistry BackofficeUserMFASecretRegistry

	// OAuthIdentityRegistry persists the link between Inventario users and
	// external OAuth provider accounts (#1394). Service-mode only — the
	// OAuth callback resolves identities before any user session exists.
	OAuthIdentityRegistry OAuthIdentityRegistry

	// SystemStats reports installation-wide entity counts and storage
	// usage for the business-metrics collector (#843). It deliberately
	// bypasses tenant scoping — the totals cover every tenant and group,
	// so it must NOT be called from any user-facing request path. It is
	// populated by the backend constructor (postgres runs the aggregate
	// reads under the background-worker role; the memory backend reports
	// zeros). Nil-able: a nil func means "no business gauges", and the
	// collector no-ops on a nil source.
	SystemStats SystemStatsFunc

	// WorkerControlRegistry holds the global background-worker soft-pause
	// control rows (issue #1308). Lives on FactorySet only — not
	// tenant-scoped, not user-aware, no RLS (same posture as
	// SystemAdminGrantRegistry / AuditLogRegistry).
	WorkerControlRegistry WorkerControlRegistry
}

// Ping checks readiness of the backing registry dependency (e.g. database).
// If no ping function is configured, it reports success.
func (fs *FactorySet) Ping(ctx context.Context) error {
	if fs == nil || fs.PingFn == nil {
		return nil
	}
	return fs.PingFn(ctx)
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

	commodityEventRegistry, err := fs.CommodityEventRegistryFactory.CreateUserRegistry(ctx)
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

	tagRegistry, err := fs.TagRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	commodityLoanRegistry, err := fs.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	commodityServiceRegistry, err := fs.CommodityServiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	supplyLinkRegistry, err := fs.SupplyLinkRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	maintenanceScheduleRegistry, err := fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry(ctx)
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

	currencyMigrationRegistry, err := fs.CurrencyMigrationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}

	return &Set{
		LocationRegistry:               locationRegistry,
		AreaRegistry:                   areaRegistry,
		CommodityRegistry:              commodityRegistry,
		CommodityEventRegistry:         commodityEventRegistry,
		SettingsRegistry:               settingsRegistry,
		ExportRegistry:                 exportRegistry,
		RestoreOperationRegistry:       restoreOperationRegistry,
		RestoreStepRegistry:            restoreStepRegistry,
		FileRegistry:                   fileRegistry,
		TagRegistry:                    tagRegistry,
		CommodityLoanRegistry:          commodityLoanRegistry,
		CommodityServiceRegistry:       commodityServiceRegistry,
		SupplyLinkRegistry:             supplyLinkRegistry,
		MaintenanceScheduleRegistry:    maintenanceScheduleRegistry,
		ThumbnailGenerationJobRegistry: thumbnailGenerationJobRegistry,
		UserConcurrencySlotRegistry:    userConcurrencySlotRegistry,
		OperationSlotRegistry:          operationSlotRegistry,
		TenantRegistry:                 fs.TenantRegistry,
		UserRegistry:                   fs.UserRegistry,
		RefreshTokenRegistry:           fs.RefreshTokenRegistry,
		LoginEventRegistry:             fs.LoginEventRegistry,
		UserMFASecretRegistry:          fs.UserMFASecretRegistry,
		AuditLogRegistry:               fs.AuditLogRegistry,
		EmailVerificationRegistry:      fs.EmailVerificationRegistry,
		PasswordResetRegistry:          fs.PasswordResetRegistry,
		LocationGroupRegistry:          fs.LocationGroupRegistry,
		GroupMembershipRegistry:        fs.GroupMembershipRegistry,
		GroupInviteRegistry:            fs.GroupInviteRegistry,
		GroupInviteAuditRegistry:       fs.GroupInviteAuditRegistry,
		GroupNotificationPrefRegistry:  fs.GroupNotificationPrefRegistry,
		GroupPurger:                    fs.GroupPurger,
		WarrantyReminderRegistry:       fs.WarrantyReminderRegistry,
		StorageQuotaReminderRegistry:   fs.StorageQuotaReminderRegistry,
		MaintenanceReminderRegistry:    fs.MaintenanceReminderRegistry,
		CurrencyMigrationRegistry:      currencyMigrationRegistry,
		CommodityScanAuditRegistry:     fs.CommodityScanAuditRegistry,
		SystemAdminGrantRegistry:       fs.SystemAdminGrantRegistry,
		OAuthIdentityRegistry:          fs.OAuthIdentityRegistry,
		WorkerControlRegistry:          fs.WorkerControlRegistry,
	}, nil
}

// CreateServiceRegistrySet creates a complete set of service-aware registries from factories
func (fs *FactorySet) CreateServiceRegistrySet() *Set {
	return &Set{
		LocationRegistry:               fs.LocationRegistryFactory.CreateServiceRegistry(),
		AreaRegistry:                   fs.AreaRegistryFactory.CreateServiceRegistry(),
		CommodityRegistry:              fs.CommodityRegistryFactory.CreateServiceRegistry(),
		CommodityEventRegistry:         fs.CommodityEventRegistryFactory.CreateServiceRegistry(),
		SettingsRegistry:               fs.SettingsRegistryFactory.CreateServiceRegistry(),
		ExportRegistry:                 fs.ExportRegistryFactory.CreateServiceRegistry(),
		RestoreOperationRegistry:       fs.RestoreOperationRegistryFactory.CreateServiceRegistry(),
		RestoreStepRegistry:            fs.RestoreStepRegistryFactory.CreateServiceRegistry(),
		FileRegistry:                   fs.FileRegistryFactory.CreateServiceRegistry(),
		TagRegistry:                    fs.TagRegistryFactory.CreateServiceRegistry(),
		CommodityLoanRegistry:          fs.CommodityLoanRegistryFactory.CreateServiceRegistry(),
		CommodityServiceRegistry:       fs.CommodityServiceRegistryFactory.CreateServiceRegistry(),
		SupplyLinkRegistry:             fs.SupplyLinkRegistryFactory.CreateServiceRegistry(),
		MaintenanceScheduleRegistry:    fs.MaintenanceScheduleRegistryFactory.CreateServiceRegistry(),
		ThumbnailGenerationJobRegistry: fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry(),
		UserConcurrencySlotRegistry:    fs.UserConcurrencySlotRegistryFactory.CreateServiceRegistry(),
		OperationSlotRegistry:          fs.OperationSlotRegistryFactory.CreateServiceRegistry(),
		TenantRegistry:                 fs.TenantRegistry,
		UserRegistry:                   fs.UserRegistry,
		RefreshTokenRegistry:           fs.RefreshTokenRegistry,
		LoginEventRegistry:             fs.LoginEventRegistry,
		UserMFASecretRegistry:          fs.UserMFASecretRegistry,
		AuditLogRegistry:               fs.AuditLogRegistry,
		EmailVerificationRegistry:      fs.EmailVerificationRegistry,
		PasswordResetRegistry:          fs.PasswordResetRegistry,
		LocationGroupRegistry:          fs.LocationGroupRegistry,
		GroupMembershipRegistry:        fs.GroupMembershipRegistry,
		GroupInviteRegistry:            fs.GroupInviteRegistry,
		GroupInviteAuditRegistry:       fs.GroupInviteAuditRegistry,
		GroupNotificationPrefRegistry:  fs.GroupNotificationPrefRegistry,
		GroupPurger:                    fs.GroupPurger,
		WarrantyReminderRegistry:       fs.WarrantyReminderRegistry,
		StorageQuotaReminderRegistry:   fs.StorageQuotaReminderRegistry,
		MaintenanceReminderRegistry:    fs.MaintenanceReminderRegistry,
		CurrencyMigrationRegistry:      fs.CurrencyMigrationRegistryFactory.CreateServiceRegistry(),
		CommodityScanAuditRegistry:     fs.CommodityScanAuditRegistry,
		SystemAdminGrantRegistry:       fs.SystemAdminGrantRegistry,
		OAuthIdentityRegistry:          fs.OAuthIdentityRegistry,
		WorkerControlRegistry:          fs.WorkerControlRegistry,
	}
}
