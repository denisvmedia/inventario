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

	// ListPaginated returns a paginated list of areas along with the total count.
	ListPaginated(ctx context.Context, offset, limit int) ([]*models.Area, int, error)
}

// CommoditySortField names the columns the commodities list endpoint
// understands for sorting. The names are part of the public API surface
// (FE codegen reads them), so add new variants conservatively.
type CommoditySortField string

const (
	CommoditySortName           CommoditySortField = "name"
	CommoditySortRegisteredDate CommoditySortField = "registered_date"
	CommoditySortPurchaseDate   CommoditySortField = "purchase_date"
	CommoditySortCurrentPrice   CommoditySortField = "current_price"
	CommoditySortOriginalPrice  CommoditySortField = "original_price"
	CommoditySortCount          CommoditySortField = "count"
)

// IsValid reports whether s is one of the known sort fields. Callers
// should fall back to CommoditySortName on invalid input rather than
// surface a 4xx — the FE may pass an unknown sort while a multi-version
// rollout is in flight.
func (s CommoditySortField) IsValid() bool {
	switch s {
	case CommoditySortName, CommoditySortRegisteredDate, CommoditySortPurchaseDate,
		CommoditySortCurrentPrice, CommoditySortOriginalPrice, CommoditySortCount:
		return true
	}
	return false
}

// CommodityListOptions narrows the result of CommodityRegistry.ListPaginated.
// Empty fields mean "no filter" — pass a zero value to get the same shape
// the old ListPaginated(ctx, offset, limit) returned. Slice filters are
// OR-ed within a field (`Types: ["white_goods", "electronics"]` matches
// either), AND-ed across fields.
type CommodityListOptions struct {
	// Types restricts the result to commodities whose Type is in the
	// list. Each value should be a valid models.CommodityType; unknown
	// values match nothing. Empty = unrestricted.
	Types []models.CommodityType
	// Statuses restricts by the Status enum (in_use, sold, lost,
	// disposed, written_off). Empty = unrestricted.
	Statuses []models.CommodityStatus
	// AreaID, when non-empty, restricts to a single area. Use "" to
	// disable the filter (rather than a sentinel like "*").
	AreaID string
	// Search runs a case-insensitive substring match against the Name
	// and ShortName fields. Empty = no search.
	Search string
	// IncludeInactive controls whether non-`in_use` commodities AND
	// drafts are included. The list page hides them by default; when
	// the user toggles "Show inactive" the FE sends true. This is
	// independent of the explicit Statuses filter — passing both is a
	// supported combination ("show drafts but only sold ones").
	IncludeInactive bool
	// SortField — see CommoditySortField. Invalid values fall back to
	// CommoditySortName silently (see IsValid).
	SortField CommoditySortField
	// SortDesc reverses the natural order of the chosen field. Default
	// false — name is ascending, prices/dates ascending too. The FE
	// sends `-name` style strings; the handler is responsible for
	// splitting the leading `-` into this bool.
	SortDesc bool
}

type CommodityRegistry interface {
	Registry[models.Commodity]

	// ListPaginated returns a paginated list of commodities along with the total count,
	// optionally filtered and sorted via opts. Pass a zero CommodityListOptions for the
	// previous "all rows, name+id ascending" behaviour.
	ListPaginated(ctx context.Context, offset, limit int, opts CommodityListOptions) ([]*models.Commodity, int, error)

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

	// ListPaginated returns a paginated list of locations along with the total count.
	ListPaginated(ctx context.Context, offset, limit int) ([]*models.Location, int, error)
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

	// ListByGroup returns every file belonging to the given (tenant_id,
	// group_id) tuple. Used by the group purge worker to find physical blobs
	// to delete before the row-level purge wipes the file table — avoids the
	// O(tenant × total_files) scan that List() would perform. Only makes
	// sense for service-mode callers: group-scoped user registries already
	// see exactly the right slice via RLS.
	ListByGroup(ctx context.Context, tenantID, groupID string) ([]*models.FileEntity, error)

	// Search returns files matching the search criteria. Optional filters:
	//   - fileCategory narrows by the user-meaningful tile category
	//     (Photos/Invoices/Documents/Other).
	//   - linkedEntityType / linkedEntityID narrow to files linked to a
	//     specific commodity/location/export. Both must be supplied together
	//     or both nil; passing only one is a programmer error and treated as
	//     "no linked-entity filter".
	Search(ctx context.Context, query string, fileType *models.FileType, fileCategory *models.FileCategory, tags []string, linkedEntityType, linkedEntityID *string) ([]*models.FileEntity, error)

	//// FullTextSearch performs enhanced text search on files
	// FullTextSearch(ctx context.Context, query string, fileType *models.FileType, options ...SearchOption) ([]*models.FileEntity, error)

	// ListPaginated returns paginated list of files. Optional filters:
	//   - fileCategory narrows by tile category.
	//   - linkedEntityType / linkedEntityID narrow to files linked to a
	//     specific commodity/location/export (both required together).
	ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType, fileCategory *models.FileCategory, linkedEntityType, linkedEntityID *string) ([]*models.FileEntity, int, error)

	// CountByCategory returns the per-category file count, scoped to the
	// current group via RLS and constrained by the same filters as Search
	// (text query, file type, tags). Backs the GET /files/category-counts
	// endpoint that drives the four-tile UI on the Files page.
	CountByCategory(ctx context.Context, query string, fileType *models.FileType, tags []string) (map[models.FileCategory]int, error)
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

	// GetDefault returns the tenant marked as default (IsDefault == true).
	// Returns ErrNotFound if no default tenant has been configured.
	GetDefault(ctx context.Context) (*models.Tenant, error)

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

// PasswordResetRegistry manages password-reset tokens.
type PasswordResetRegistry interface {
	Registry[models.PasswordReset]

	// GetByToken returns a password-reset record by its token value.
	GetByToken(ctx context.Context, token string) (*models.PasswordReset, error)

	// GetByUserID returns all password-reset records belonging to the given user.
	GetByUserID(ctx context.Context, userID string) ([]*models.PasswordReset, error)

	// DeleteByUserID removes all password-reset records for the given user.
	DeleteByUserID(ctx context.Context, userID string) error

	// DeleteExpired removes all records whose ExpiresAt timestamp is in the past.
	DeleteExpired(ctx context.Context) error
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

// LocationGroupRegistry manages location groups within a tenant.
// Groups are tenant-scoped (not user-scoped) — access is controlled via memberships.
type LocationGroupRegistry interface {
	Registry[models.LocationGroup]

	// GetBySlug returns a group by its slug within a tenant.
	GetBySlug(ctx context.Context, tenantID, slug string) (*models.LocationGroup, error)

	// ListByTenant returns all groups for a tenant.
	ListByTenant(ctx context.Context, tenantID string) ([]*models.LocationGroup, error)
}

// GroupMembershipRegistry manages user memberships in location groups.
type GroupMembershipRegistry interface {
	Registry[models.GroupMembership]

	// GetByGroupAndUser returns a membership for a specific user in a specific group.
	GetByGroupAndUser(ctx context.Context, groupID, userID string) (*models.GroupMembership, error)

	// ListByGroup returns all memberships for a group.
	ListByGroup(ctx context.Context, groupID string) ([]*models.GroupMembership, error)

	// ListByUser returns all memberships for a user within a tenant.
	ListByUser(ctx context.Context, tenantID, userID string) ([]*models.GroupMembership, error)

	// CountAdminsByGroup returns the number of admins in a group.
	CountAdminsByGroup(ctx context.Context, groupID string) (int, error)
}

// GroupInviteRegistry manages invite links for location groups.
type GroupInviteRegistry interface {
	Registry[models.GroupInvite]

	// GetByToken returns an invite by its token.
	GetByToken(ctx context.Context, token string) (*models.GroupInvite, error)

	// ListActiveByGroup returns all non-expired, unused invites for a group.
	ListActiveByGroup(ctx context.Context, groupID string) ([]*models.GroupInvite, error)

	// ListUsedByGroup returns every invite belonging to the given group that
	// has already been accepted (used_by IS NOT NULL). Called by the group
	// purge worker to build the audit snapshot without having to page through
	// the whole invite table. Implementations run in service mode and ignore
	// tenant RLS; callers must supply a group ID they are authorised to purge.
	ListUsedByGroup(ctx context.Context, groupID string) ([]*models.GroupInvite, error)

	// MarkUsed atomically marks an invite as used by the given user.
	// It returns (true, nil) iff this call was the winner of the compare-and-swap
	// and mutated the row. A previously-used invite returns (false, nil); other
	// errors return (false, err). Implementations must guarantee that at most
	// one concurrent caller succeeds per invite — postgres uses a conditional
	// UPDATE, memory uses a mutex.
	MarkUsed(ctx context.Context, inviteID, userID string, usedAt time.Time) (bool, error)

	// DeleteByGroup removes all invite rows (used or unused) belonging to the
	// given group. Called by the group purge worker right after it snapshots
	// used invites into the audit table. Returns the number of deleted rows.
	DeleteByGroup(ctx context.Context, groupID string) (int, error)

	// DeleteExpiredUnused removes every invite whose ExpiresAt is before the
	// provided cutoff and that has not been accepted (used_by IS NULL).
	// Returns the number of deleted rows. Used by the housekeeping expiry
	// sweep (spec #1309 Option 2i).
	DeleteExpiredUnused(ctx context.Context, cutoff time.Time) (int, error)
}

// GroupInviteAuditRegistry manages persistent audit rows for used invites
// that outlive their parent LocationGroup. Rows are inserted only by the
// group purge worker and are tenant-scoped (no group FK — the source group
// is hard-deleted as part of the purge).
type GroupInviteAuditRegistry interface {
	Registry[models.GroupInviteAudit]

	// ListByOriginalGroup returns all audit records for a previously-purged
	// group, identified by its original (pre-purge) group ID.
	ListByOriginalGroup(ctx context.Context, originalGroupID string) ([]*models.GroupInviteAudit, error)

	// ListByTenant returns all audit records for a tenant, most recent first.
	ListByTenant(ctx context.Context, tenantID string) ([]*models.GroupInviteAudit, error)
}

type UserRegistry interface {
	Registry[models.User]

	// GetByEmail returns a user by email within a tenant
	GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error)

	// ListByTenant returns all users for a tenant
	ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error)
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

// GroupPurger hard-deletes every row whose group_id references the given
// LocationGroup, in a FK-safe order: restore_steps, restore_operations,
// exports, commodities, files, areas, locations and finally group_memberships.
// (The legacy commodity-scoped images/invoices/manuals tables were dropped
// under #1421 — their data lives in `files` now.) It is intentionally a separate abstraction
// from per-registry CRUD because the purge flow must run under the
// background-worker RLS role and cross many entity boundaries in a single
// transaction.
//
// The LocationGroup row itself and any group_invites / group_invites_audit
// rows are NOT touched here — the caller (GroupPurgeService) handles invite
// snapshotting and the final location_groups DELETE separately so blob
// cleanup, audit-writing and group removal remain explicit at the
// orchestration layer.
type GroupPurger interface {
	// PurgeGroupDependents deletes all dependent entities for the given
	// tenant/group pair. Implementations must be idempotent — a second call
	// on the same group after a partial failure must succeed and leave the
	// database in the same state.
	PurgeGroupDependents(ctx context.Context, tenantID, groupID string) error
}

// Set contains ready-to-use registries that have been created with proper user or service context.
// This is the result of calling CreateUserRegistrySet() or CreateServiceRegistrySet() on a FactorySet.
type Set struct {
	LocationRegistry               LocationRegistry
	AreaRegistry                   AreaRegistry
	CommodityRegistry              CommodityRegistry
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
	PasswordResetRegistry          PasswordResetRegistry     // PasswordResetRegistry doesn't need factory as it's not user-aware
	LocationGroupRegistry          LocationGroupRegistry     // LocationGroupRegistry is tenant-scoped, not user-aware
	GroupMembershipRegistry        GroupMembershipRegistry   // GroupMembershipRegistry is tenant-scoped, not user-aware
	GroupInviteRegistry            GroupInviteRegistry       // GroupInviteRegistry is tenant-scoped, not user-aware
	GroupInviteAuditRegistry       GroupInviteAuditRegistry  // GroupInviteAuditRegistry is tenant-scoped, not user-aware; written only by the group purge worker
	GroupPurger                    GroupPurger               // GroupPurger bulk-removes group-scoped entities during the purge worker's tick
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
		validation.Field(&s.SettingsRegistry, validation.Required),
		validation.Field(&s.ExportRegistry, validation.Required),
		validation.Field(&s.FileRegistry, validation.Required),
		validation.Field(&s.TenantRegistry, validation.Required),
		validation.Field(&s.UserRegistry, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, s, fields...)
}
