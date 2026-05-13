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
	ThumbnailGenerationJobRegistryFactory ThumbnailGenerationJobRegistryFactory
	UserConcurrencySlotRegistryFactory    UserConcurrencySlotRegistryFactory
	OperationSlotRegistryFactory          OperationSlotRegistryFactory
	PingFn                                func(context.Context) error   // Optional health check hook for backing storage.
	TenantRegistry                        TenantRegistry                // TenantRegistry doesn't need factory as it's not user-aware
	UserRegistry                          UserRegistry                  // UserRegistry doesn't need factory as it's not user-aware
	RefreshTokenRegistry                  RefreshTokenRegistry          // RefreshTokenRegistry doesn't need factory as it's not user-aware
	AuditLogRegistry                      AuditLogRegistry              // AuditLogRegistry doesn't need factory as it's not user-aware
	EmailVerificationRegistry             EmailVerificationRegistry     // EmailVerificationRegistry doesn't need factory as it's not user-aware
	PasswordResetRegistry                 PasswordResetRegistry         // PasswordResetRegistry doesn't need factory as it's not user-aware
	LocationGroupRegistry                 LocationGroupRegistry         // LocationGroupRegistry is tenant-scoped, not user-aware
	GroupMembershipRegistry               GroupMembershipRegistry       // GroupMembershipRegistry is tenant-scoped, not user-aware
	GroupInviteRegistry                   GroupInviteRegistry           // GroupInviteRegistry is tenant-scoped, not user-aware
	GroupInviteAuditRegistry              GroupInviteAuditRegistry      // GroupInviteAuditRegistry is tenant-scoped, not user-aware
	GroupNotificationPrefRegistry         GroupNotificationPrefRegistry // Per-group notification opt-outs (#1648); tenant-scoped, user-filtered in application logic
	GroupPurger                           GroupPurger                   // GroupPurger hard-deletes group-scoped data during purge ticks
	WarrantyReminderRegistry              WarrantyReminderRegistry      // WarrantyReminderRegistry is the worker idempotency store; service-mode only
	CurrencyMigrationRegistryFactory      CurrencyMigrationRegistryFactory
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
		ThumbnailGenerationJobRegistry: thumbnailGenerationJobRegistry,
		UserConcurrencySlotRegistry:    userConcurrencySlotRegistry,
		OperationSlotRegistry:          operationSlotRegistry,
		TenantRegistry:                 fs.TenantRegistry,
		UserRegistry:                   fs.UserRegistry,
		RefreshTokenRegistry:           fs.RefreshTokenRegistry,
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
		CurrencyMigrationRegistry:      currencyMigrationRegistry,
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
		ThumbnailGenerationJobRegistry: fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry(),
		UserConcurrencySlotRegistry:    fs.UserConcurrencySlotRegistryFactory.CreateServiceRegistry(),
		OperationSlotRegistry:          fs.OperationSlotRegistryFactory.CreateServiceRegistry(),
		TenantRegistry:                 fs.TenantRegistry,
		UserRegistry:                   fs.UserRegistry,
		RefreshTokenRegistry:           fs.RefreshTokenRegistry,
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
		CurrencyMigrationRegistry:      fs.CurrencyMigrationRegistryFactory.CreateServiceRegistry(),
	}
}
