package registry

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

type PIDable[T any] interface {
	*T
	IDable
}

type IDable interface {
	GetID() string
	SetID(id string)
}

type Registry[T any] interface {
	// Create creates a new T in the registry.
	Create(context.Context, T) (*T, error)

	// Get returns a T from the registry.
	Get(ctx context.Context, id string) (*T, error)

	// List returns a list of Ts from the registry.
	List(context.Context) ([]*T, error)

	// Update updates a T in the registry.
	Update(context.Context, T) (*T, error)

	// Delete deletes a T from the registry.
	Delete(ctx context.Context, id string) error

	// Count returns the number of Ts in the registry.
	Count(context.Context) (int, error)
}

// Factory interfaces for creating context-aware registries
// These replace the unsafe UserAwareRegistry and ServiceAwareRegistry patterns

type UserRegistryFactory[T any, P Registry[T]] interface {
	// CreateUserRegistry creates a new registry with user context from the provided context
	CreateUserRegistry(ctx context.Context) (P, error)
	// MustCreateUserRegistry creates a new registry with user context, panics on error
	MustCreateUserRegistry(ctx context.Context) P
}

type ServiceRegistryFactory[T any, P Registry[T]] interface {
	// CreateServiceRegistry creates a new registry with service account context
	CreateServiceRegistry() P
}

type AreaRegistry interface {
	Registry[models.Area]

	GetCommodities(ctx context.Context, areaID string) ([]string, error)
}

type CommodityRegistry interface {
	Registry[models.Commodity]

	GetImages(ctx context.Context, commodityID string) ([]string, error)
	GetManuals(ctx context.Context, commodityID string) ([]string, error)
	GetInvoices(ctx context.Context, commodityID string) ([]string, error)

	// Enhanced search methods
	// SearchByTags(ctx context.Context, tags []string, operator TagOperator) ([]*models.Commodity, error)
	// FullTextSearch(ctx context.Context, query string, options ...SearchOption) ([]*models.Commodity, error)
	// FindSimilar(ctx context.Context, commodityID string, threshold float64) ([]*models.Commodity, error)
	// AggregateByArea(ctx context.Context, groupBy []string) ([]AggregationResult, error)
	// CountByStatus(ctx context.Context) (map[string]int, error)
	// CountByType(ctx context.Context) (map[string]int, error)
	// FindByPriceRange(ctx context.Context, minPrice, maxPrice float64, currency string) ([]*models.Commodity, error)
	// FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Commodity, error)
	// FindBySerialNumbers(ctx context.Context, serialNumbers []string) ([]*models.Commodity, error)
}

type LocationRegistry interface {
	Registry[models.Location]

	GetAreas(ctx context.Context, locationID string) ([]string, error)
}

type ImageRegistry interface {
	Registry[models.Image]
}

type InvoiceRegistry interface {
	Registry[models.Invoice]
}

type ManualRegistry interface {
	Registry[models.Manual]
}

type SettingsRegistry interface {
	Get(ctx context.Context) (models.SettingsObject, error)
	Save(context.Context, models.SettingsObject) error
	Patch(ctx context.Context, configfield string, value any) error
}

type ExportRegistry interface {
	Registry[models.Export]

	// ListWithDeleted returns all exports including soft deleted ones
	ListWithDeleted(ctx context.Context) ([]*models.Export, error)

	// ListDeleted returns only soft deleted exports
	ListDeleted(ctx context.Context) ([]*models.Export, error)

	// HardDelete permanently deletes an export from the database
	HardDelete(ctx context.Context, id string) error
}

type FileRegistry interface {
	Registry[models.FileEntity]

	// ListByType returns files filtered by type
	ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error)

	// ListByLinkedEntity returns files linked to a specific entity
	ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error)

	// ListByLinkedEntityAndMeta returns files linked to a specific entity with specific metadata
	ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, meta string) ([]*models.FileEntity, error)

	// Search returns files matching the search criteria
	Search(ctx context.Context, query string, fileType *models.FileType, tags []string) ([]*models.FileEntity, error)

	//// FullTextSearch performs enhanced text search on files
	// FullTextSearch(ctx context.Context, query string, fileType *models.FileType, options ...SearchOption) ([]*models.FileEntity, error)

	// ListPaginated returns paginated list of files
	ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType) ([]*models.FileEntity, int, error)
}

type ThumbnailGenerationJobRegistry interface {
	Registry[models.ThumbnailGenerationJob]

	// GetPendingJobs returns pending thumbnail generation jobs ordered by priority and creation time
	GetPendingJobs(ctx context.Context, limit int) ([]*models.ThumbnailGenerationJob, error)

	// GetJobByFileID returns the thumbnail generation job for a specific file
	GetJobByFileID(ctx context.Context, fileID string) (*models.ThumbnailGenerationJob, error)

	// UpdateJobStatus updates the status of a thumbnail generation job
	UpdateJobStatus(ctx context.Context, jobID string, status models.ThumbnailGenerationStatus, errorMessage string) error

	// CleanupCompletedJobs removes completed/failed jobs older than the specified duration
	CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) error
}

type UserConcurrencySlotRegistry interface {
	Registry[models.UserConcurrencySlot]

	// AcquireSlot attempts to acquire a concurrency slot for a user
	AcquireSlot(ctx context.Context, userID, jobID string, maxSlots int, slotDuration time.Duration) (*models.UserConcurrencySlot, error)

	// ReleaseSlot releases a concurrency slot
	ReleaseSlot(ctx context.Context, userID, jobID string) error

	// GetUserSlots returns all slots for a user
	GetUserSlots(ctx context.Context, userID string) ([]*models.UserConcurrencySlot, error)

	// CleanupExpiredSlots removes expired slots
	CleanupExpiredSlots(ctx context.Context) error
}

type OperationSlotRegistry interface {
	Registry[models.OperationSlot]

	// GetSlot retrieves a specific slot for a user and operation
	GetSlot(ctx context.Context, userID, operationName string, slotID int) (*models.OperationSlot, error)

	// ReleaseSlot removes a specific slot for a user and operation
	ReleaseSlot(ctx context.Context, userID, operationName string, slotID int) error

	// GetActiveSlotCount returns the number of active (non-expired) slots for a user and operation
	GetActiveSlotCount(ctx context.Context, userID, operationName string) (int, error)

	// GetNextSlotID returns the next available slot ID for a user and operation
	GetNextSlotID(ctx context.Context, userID, operationName string) (int, error)

	// CleanupExpiredSlots removes all expired slots and returns the count of deleted slots
	CleanupExpiredSlots(ctx context.Context) (int, error)

	// GetOperationStats returns statistics about slot usage across all operations
	GetOperationStats(ctx context.Context) (map[string]models.OperationStats, error)

	// GetUserSlotStats returns slot usage statistics for a specific user
	GetUserSlotStats(ctx context.Context, userID string) (map[string]int, error)

	// GetExpiredSlots returns all expired slots (for testing/debugging)
	GetExpiredSlots(ctx context.Context) ([]models.OperationSlot, error)
}

type RestoreOperationRegistry interface {
	Registry[models.RestoreOperation]

	// ListByExport returns all restore operations for an export
	ListByExport(ctx context.Context, exportID string) ([]*models.RestoreOperation, error)
}

type RestoreStepRegistry interface {
	Registry[models.RestoreStep]

	// ListByRestoreOperation returns all restore steps for a restore operation
	ListByRestoreOperation(ctx context.Context, restoreOperationID string) ([]*models.RestoreStep, error)

	// DeleteByRestoreOperation deletes all restore steps for a restore operation
	DeleteByRestoreOperation(ctx context.Context, restoreOperationID string) error
}

type TenantRegistry interface {
	Registry[models.Tenant]

	// GetBySlug returns a tenant by its slug
	GetBySlug(ctx context.Context, slug string) (*models.Tenant, error)

	// GetByDomain returns a tenant by its domain
	GetByDomain(ctx context.Context, domain string) (*models.Tenant, error)
}

// AuditLogRegistry manages security-relevant event records for compliance and debugging.
type AuditLogRegistry interface {
	Registry[models.AuditLog]

	// ListByUser returns all audit logs for a specific user.
	ListByUser(ctx context.Context, userID string) ([]*models.AuditLog, error)

	// ListByTenant returns all audit logs for a specific tenant.
	ListByTenant(ctx context.Context, tenantID string) ([]*models.AuditLog, error)

	// ListByAction returns all audit logs matching the given action string.
	ListByAction(ctx context.Context, action string) ([]*models.AuditLog, error)

	// DeleteOlderThan removes all audit log entries with a timestamp before cutoff.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) error
}

// EmailVerificationRegistry manages email address verification tokens.
type EmailVerificationRegistry interface {
	Registry[models.EmailVerification]

	// GetByToken returns an email verification record by its token value.
	GetByToken(ctx context.Context, token string) (*models.EmailVerification, error)

	// GetByUserID returns all email verification records for a user.
	GetByUserID(ctx context.Context, userID string) ([]*models.EmailVerification, error)

	// DeleteExpired removes all records whose expiry time has passed.
	DeleteExpired(ctx context.Context) error
}

type UserRegistry interface {
	Registry[models.User]

	// GetByEmail returns a user by email within a tenant
	GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error)

	// ListByTenant returns all users for a tenant
	ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error)

	// ListByRole returns all users with a specific role within a tenant
	ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error)
}

type RefreshTokenRegistry interface {
	Registry[models.RefreshToken]

	// GetByTokenHash returns a refresh token by its SHA-256 hash
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)

	// GetByUserID returns all refresh tokens for a user
	GetByUserID(ctx context.Context, userID string) ([]*models.RefreshToken, error)

	// RevokeByUserID marks all refresh tokens for a user as revoked
	RevokeByUserID(ctx context.Context, userID string) error

	// DeleteExpired removes all expired refresh tokens from the store
	DeleteExpired(ctx context.Context) error
}

// Set contains ready-to-use registries that have been created with proper user or service context.
// This is the result of calling CreateUserRegistrySet() or CreateServiceRegistrySet() on a FactorySet.
type Set struct {
	LocationRegistry               LocationRegistry
	AreaRegistry                   AreaRegistry
	CommodityRegistry              CommodityRegistry
	ImageRegistry                  ImageRegistry
	InvoiceRegistry                InvoiceRegistry
	ManualRegistry                 ManualRegistry
	SettingsRegistry               SettingsRegistry
	ExportRegistry                 ExportRegistry
	RestoreOperationRegistry       RestoreOperationRegistry
	RestoreStepRegistry            RestoreStepRegistry
	FileRegistry                   FileRegistry
	ThumbnailGenerationJobRegistry ThumbnailGenerationJobRegistry
	UserConcurrencySlotRegistry    UserConcurrencySlotRegistry
	OperationSlotRegistry          OperationSlotRegistry
	TenantRegistry                 TenantRegistry
	UserRegistry                   UserRegistry
	RefreshTokenRegistry           RefreshTokenRegistry
	AuditLogRegistry               AuditLogRegistry          // AuditLogRegistry doesn't need factory as it's not user-aware
	EmailVerificationRegistry      EmailVerificationRegistry // EmailVerificationRegistry doesn't need factory as it's not user-aware
}

// Search-related types and functions

// TagOperator defines how tags should be matched
type TagOperator string

const (
	TagOperatorAND TagOperator = "AND"
	TagOperatorOR  TagOperator = "OR"
)

// SearchOptions contains options for search operations
type SearchOptions struct {
	Limit  int
	Offset int
}

// SearchOption is a function that modifies SearchOptions
type SearchOption func(*SearchOptions)

// WithLimit sets the limit for search results
func WithLimit(limit int) SearchOption {
	return func(opts *SearchOptions) {
		opts.Limit = limit
	}
}

// WithOffset sets the offset for search results
func WithOffset(offset int) SearchOption {
	return func(opts *SearchOptions) {
		opts.Offset = offset
	}
}

// AggregationResult represents the result of an aggregation query
type AggregationResult struct {
	GroupBy map[string]any     `json:"group_by"`
	Count   int                `json:"count"`
	Avg     map[string]float64 `json:"avg,omitempty"`
	Sum     map[string]float64 `json:"sum,omitempty"`
	Min     map[string]float64 `json:"min,omitempty"`
	Max     map[string]float64 `json:"max,omitempty"`
}

// UserIDFromContext extracts the user ID from the context
func UserIDFromContext(ctx context.Context) string {
	return appctx.UserIDFromContext(ctx)
}

func (s *Set) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&s.LocationRegistry, validation.Required),
		validation.Field(&s.AreaRegistry, validation.Required),
		validation.Field(&s.CommodityRegistry, validation.Required),
		validation.Field(&s.ImageRegistry, validation.Required),
		validation.Field(&s.ManualRegistry, validation.Required),
		validation.Field(&s.InvoiceRegistry, validation.Required),
		validation.Field(&s.SettingsRegistry, validation.Required),
		validation.Field(&s.ExportRegistry, validation.Required),
		validation.Field(&s.FileRegistry, validation.Required),
		validation.Field(&s.TenantRegistry, validation.Required),
		validation.Field(&s.UserRegistry, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, s, fields...)
}
