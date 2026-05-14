package registry

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

// WarrantyStatusFilter is the warranty filter accepted by
// CommodityRegistry.ListPaginated. Mirrors models.WarrantyStatus 1:1 but
// re-declared here so the registry layer doesn't transitively depend on
// the FE-facing constants — the API handler is responsible for
// translating models.WarrantyStatus values into this type.
type WarrantyStatusFilter string

const (
	WarrantyStatusFilterNone     WarrantyStatusFilter = "none"
	WarrantyStatusFilterActive   WarrantyStatusFilter = "active"
	WarrantyStatusFilterExpiring WarrantyStatusFilter = "expiring"
	WarrantyStatusFilterExpired  WarrantyStatusFilter = "expired"
)

// IsValid reports whether s is one of the documented warranty filter
// values. Empty string is treated as "no filter" by callers and is not
// considered valid here.
func (s WarrantyStatusFilter) IsValid() bool {
	switch s {
	case WarrantyStatusFilterNone, WarrantyStatusFilterActive, WarrantyStatusFilterExpiring, WarrantyStatusFilterExpired:
		return true
	}
	return false
}

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

	// ListPaginated returns a paginated list of areas along with the total
	// count, optionally filtered via opts. Pass a zero AreaListOptions for
	// the unfiltered shape the old `(ctx, offset, limit)` form returned.
	ListPaginated(ctx context.Context, offset, limit int, opts AreaListOptions) ([]*models.Area, int, error)
}

// AreaListOptions narrows the result of AreaRegistry.ListPaginated. Empty
// fields mean "no filter" — the zero value yields the same shape as an
// unfiltered listing.
type AreaListOptions struct {
	// LocationID, when non-empty, restricts the result to areas inside a
	// single location. Use "" to disable the filter (rather than a sentinel
	// like "*"). An unknown ID matches nothing — RLS already group-scopes
	// the query, so a cross-tenant ID returns the empty list rather than a
	// 4xx.
	LocationID string
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
	// WarrantyStatuses, when non-empty, restricts the result to
	// commodities whose computed warranty status is in the list.
	// Computation is `models.ComputeWarrantyStatus(WarrantyExpiresAt,
	// WarrantyNow)`. The implementations evaluate the predicate against
	// the same `WarrantyNow` timestamp to keep the result deterministic
	// for the duration of the request.
	WarrantyStatuses []WarrantyStatusFilter
	// WarrantyExpiresBefore filters out commodities whose
	// WarrantyExpiresAt is at or after the given date. Empty = no
	// filter. Combined with WarrantyStatuses via AND. Format is
	// YYYY-MM-DD (matching PDate's wire format) and the comparison is
	// lexicographic, which is correct for ISO dates.
	WarrantyExpiresBefore string
	// WarrantyNow is the server clock used by warranty filters. Pass
	// time.Time{} to mean "use real now"; tests pass a frozen value so
	// status computations are deterministic. Implementations only
	// consult it when WarrantyStatuses is non-empty.
	WarrantyNow time.Time
}

// CommodityEventListOptions narrows the result of CommodityEventRegistry.ListByCommodity.
// Empty fields mean "no filter".
type CommodityEventListOptions struct {
	// Kinds, when non-empty, restricts the result to events whose Kind is
	// in the list. Unknown values match nothing. Empty = unrestricted.
	Kinds []models.CommodityEventKind
}

// CommodityEventRegistry is the append-only audit log of commodity state
// changes (issue #1450). Writes happen at the apiserver layer right after
// a successful CRUD; reads back the timeline newest-first for the detail
// page's history rail.
type CommodityEventRegistry interface {
	Registry[models.CommodityEvent]

	// ListByCommodity returns paginated events for the given commodity,
	// newest first. Total reflects the filtered count (post-Kinds, pre-LIMIT).
	ListByCommodity(ctx context.Context, commodityID string, offset, limit int, opts CommodityEventListOptions) ([]*models.CommodityEvent, int, error)
}

type CommodityRegistry interface {
	Registry[models.Commodity]

	// ListPaginated returns a paginated list of commodities along with the total count,
	// optionally filtered and sorted via opts. Pass a zero CommodityListOptions for the
	// previous "all rows, name+id ascending" behaviour.
	ListPaginated(ctx context.Context, offset, limit int, opts CommodityListOptions) ([]*models.Commodity, int, error)

	// ListByGroup returns every commodity in the given (tenant_id, group_id)
	// tuple, regardless of draft / status. Used by the currency-migration
	// service (issue #202): the conversion needs to see all rows in the
	// group, not just the user-visible "in_use, non-draft" subset that
	// ListPaginated defaults to. Service-mode callers (the worker) bypass
	// RLS via this; user-mode callers should use the registry's group
	// context instead.
	ListByGroup(ctx context.Context, tenantID, groupID string) ([]*models.Commodity, error)

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
	//     (Images/Invoices/Documents/Other).
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

	// CountByCategory returns the per-category file count and total byte
	// size, scoped to the current group via RLS and constrained by the
	// same filters as Search (text query, file type, tags). Backs the GET
	// /files/category-counts endpoint that drives the four-tile UI and the
	// cumulative footer on the Files page.
	CountByCategory(ctx context.Context, query string, fileType *models.FileType, tags []string) (map[models.FileCategory]int, map[models.FileCategory]int64, error)

	// SumSizeBreakdown returns per-bucket byte totals for the current
	// (tenant, group) scope. Backs GET /g/{slug}/storage-usage (#1388).
	// Export bundles are split out from the FileCategoryOther bucket
	// because they aren't user-facing files in the four-tile UI; the
	// quota visualization lists them as a distinct row.
	SumSizeBreakdown(ctx context.Context) (StorageBreakdown, error)

	// ListPendingSizeBackfill streams up to limit file rows whose
	// size_bytes is still zero — the rows that pre-date #1388 and need
	// the boot-time backfill to re-stat the blob and write the actual
	// size. Service-mode only; the backfill runs across every tenant
	// and group, bypassing RLS. Implementations may return fewer rows
	// than limit (the queue is exhausted) but must never return more.
	ListPendingSizeBackfill(ctx context.Context, limit int) ([]*models.FileEntity, error)
}

// StorageBreakdown is the per-bucket byte count returned by
// FileRegistry.SumSizeBreakdown. Images / Invoices / Documents / Other
// mirror models.FileCategory; Exports is files where
// linked_entity_type='export' (export bundles, removed from Other to
// keep the user-meaningful tile semantics intact).
type StorageBreakdown struct {
	Images    int64 `json:"images"`
	Invoices  int64 `json:"invoices"`
	Documents int64 `json:"documents"`
	Other     int64 `json:"other"`
	Exports   int64 `json:"exports"`
}

// Total returns the sum of every bucket. Convenience for callers that
// want the headline number alongside the breakdown.
func (b StorageBreakdown) Total() int64 {
	return b.Images + b.Invoices + b.Documents + b.Other + b.Exports
}

// TagSortField names the columns the tags list endpoint understands for
// sorting. Names are part of the public API surface (FE codegen reads them).
type TagSortField string

const (
	TagSortLabel     TagSortField = "label"
	TagSortCreatedAt TagSortField = "created_at"
	TagSortUsage     TagSortField = "usage"
)

// IsValid reports whether s is a known tag sort field. Callers should fall
// back to TagSortLabel on invalid input rather than 4xx — the FE may pass an
// unknown sort during a multi-version rollout.
func (s TagSortField) IsValid() bool {
	switch s {
	case TagSortLabel, TagSortCreatedAt, TagSortUsage:
		return true
	}
	return false
}

// TagListOptions narrows the result of TagRegistry.ListPaginated.
type TagListOptions struct {
	// Search runs case-insensitive substring match on label and slug.
	Search string
	// SortField — invalid values fall back to TagSortLabel silently.
	SortField TagSortField
	// SortDesc reverses the natural order of the chosen field.
	SortDesc bool
}

// TagUsage is the per-tag breakdown of how many commodity / file rows
// reference it via their JSONB tags array. Computed on demand; not
// denormalized onto the tags row itself.
type TagUsage struct {
	Commodities int `json:"commodities"`
	Files       int `json:"files"`
}

// TagStats is the group-wide tag adoption summary that backs the Tags page
// stats bar. Tagged/untagged counts are derived from the JSONB tags array
// on commodities + files (presence vs. emptiness, not per-tag breakdown).
type TagStats struct {
	TagsTotal     int `json:"tags_total"`
	ItemsTagged   int `json:"items_tagged"`
	ItemsUntagged int `json:"items_untagged"`
	FilesTagged   int `json:"files_tagged"`
	FilesUntagged int `json:"files_untagged"`
}

// TagRegistry is the group-scoped catalogue of tags. The tag-string
// associations themselves continue to live in JSONB on commodities/files —
// only the metadata (label, color, usage) lives here.
type TagRegistry interface {
	Registry[models.Tag]

	// GetBySlug returns a tag by its slug within the current group.
	// Returns ErrNotFound if no tag with that slug exists.
	GetBySlug(ctx context.Context, slug string) (*models.Tag, error)

	// ListPaginated returns paginated tags with optional q-search and
	// sorting. Pass a zero TagListOptions for "all rows, label asc".
	ListPaginated(ctx context.Context, offset, limit int, opts TagListOptions) ([]*models.Tag, int, error)

	// Search returns tags whose label or slug matches q (case-insensitive
	// substring), capped at limit. Used by the autocomplete endpoint and
	// ranked by usage desc + recency. Empty q returns the most-recently
	// used tags, also capped at limit.
	Search(ctx context.Context, q string, limit int) ([]*models.Tag, error)

	// GetUsage returns the per-entity reference counts for a tag slug
	// within the current group (commodities + files JSONB arrays
	// containing this slug).
	GetUsage(ctx context.Context, slug string) (TagUsage, error)

	// GetUsageBatch returns per-slug usage for the given slugs in a single
	// round-trip. Used by the GET /tags?include=usage list endpoint to
	// avoid N+1 queries when the FE wants per-row usage for the whole
	// page. Returned map is keyed by slug; missing slugs map to a zero
	// TagUsage so callers can read it unconditionally.
	GetUsageBatch(ctx context.Context, slugs []string) (map[string]TagUsage, error)

	// GetStats returns the group-wide adoption summary for the Tags page
	// stats bar: total tags, plus tagged/untagged counts on commodities
	// and files. "Tagged" = jsonb_array_length(tags) > 0.
	GetStats(ctx context.Context) (TagStats, error)

	// RewriteSlugReferences atomically rewrites every commodities.tags +
	// files.tags JSONB array entry from oldSlug to newSlug for the
	// current group, in the same logical operation as the slug change on
	// the tags row itself. Returns (commodityRows, fileRows) touched.
	RewriteSlugReferences(ctx context.Context, oldSlug, newSlug string) (int, int, error)

	// StripSlugReferences atomically removes every occurrence of slug
	// from commodities.tags + files.tags JSONB arrays for the current
	// group. Used by force-delete. Returns (commodityRows, fileRows).
	StripSlugReferences(ctx context.Context, slug string) (int, int, error)

	// RenameAtomic re-reads the tag, validates the new slug isn't already
	// owned by another tag, rewrites every JSONB reference, and writes
	// the updated tags row — all under a single advisory lock on the tag
	// id so two parallel renames of the same tag can't end with the
	// JSONB references and the tags row pointing at different slugs.
	//
	// The slug used as the rewrite source is whatever the row holds at
	// lock-acquisition time, not whatever the caller saw before this
	// call: a previous concurrent rename moves the tag forward, and the
	// next rename starts from there.
	//
	// Pass newSlug == "" to keep the existing slug. Empty newLabel /
	// newColor leave the corresponding fields untouched.
	RenameAtomic(ctx context.Context, id, newLabel, newSlug string, newColor models.TagColor) (*models.Tag, error)

	// DeleteAtomic re-checks usage, strips JSONB references (when
	// force=true), and deletes the tags row — all under a single
	// advisory lock, so a concurrent commodity insert that would
	// otherwise leak an orphan JSONB reference serializes against this
	// operation instead.
	//
	// When force=false and usage > 0, returns the usage breakdown
	// alongside registry.ErrTagInUse (defined in registry/errors.go)
	// without mutating any state. Callers compare via errors.Is.
	//
	//revive:disable-next-line:flag-parameter
	DeleteAtomic(ctx context.Context, id string, force bool) (TagUsage, error)
}

// LoanState narrows the result of CommodityLoanRegistry.ListPaginated.
// The filter is part of the public API surface (the FE list-page sends
// it as ?state=); add new variants conservatively. "all" is the
// idiomatic "no filter" sentinel — the empty string also means "all"
// to keep handler parsing terse.
type LoanState string

const (
	// LoanStateAll matches every row. Default when ?state= is missing.
	LoanStateAll LoanState = "all"
	// LoanStateOpen matches loans where returned_at IS NULL.
	LoanStateOpen LoanState = "open"
	// LoanStateOverdue matches OPEN loans whose due_back_at is set and
	// before today (server clock — same `now` as IsOverdue uses).
	LoanStateOverdue LoanState = "overdue"
	// LoanStateReturned matches loans where returned_at IS NOT NULL.
	LoanStateReturned LoanState = "returned"
)

// IsValid reports whether s is one of the known loan states. Empty
// string is intentionally treated as valid + equivalent to "all" by
// callers, so this returns false on "" — handlers can fall back to
// LoanStateAll explicitly rather than rely on the validator.
func (s LoanState) IsValid() bool {
	switch s {
	case LoanStateAll, LoanStateOpen, LoanStateOverdue, LoanStateReturned:
		return true
	}
	return false
}

// LoanListOptions narrows the result of CommodityLoanRegistry.ListPaginated.
type LoanListOptions struct {
	// State filters by loan state. Empty (or LoanStateAll) returns all.
	State LoanState
	// Now is the server clock used to evaluate LoanStateOverdue. Pass
	// time.Time{} for "use real now"; tests pass a frozen value.
	Now time.Time
}

// CommodityLoanRegistry is the group-scoped registry of commodity_loans.
// Loans are simple row-based entities — there are no cross-entity helpers
// (no JSONB rewrite, no advisory locks): every commodity has its own
// single open-loan row at most, enforced by the service layer via a
// "fetch open + reject if exists" check rather than a partial-unique
// constraint (the FE needs a domain 409, not a Postgres SQLState).
type CommodityLoanRegistry interface {
	Registry[models.CommodityLoan]

	// ListByCommodity returns all loans (open + closed) for a single
	// commodity, ordered most-recent-first. Used by the per-item Lend
	// tab to render current loan + history.
	ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityLoan, error)

	// GetOpenForCommodity returns the (at most one) open loan for a
	// commodity, or registry.ErrNotFound if there isn't one. Used by
	// the service layer's invariant check on Create.
	GetOpenForCommodity(ctx context.Context, commodityID string) (*models.CommodityLoan, error)

	// ListPaginated returns a paginated, group-wide list of loans
	// filtered by state, ordered by lent_at desc. Pass a zero
	// LoanListOptions for "all rows".
	ListPaginated(ctx context.Context, offset, limit int, opts LoanListOptions) ([]*models.CommodityLoan, int, error)

	// CountOpenByCommodity returns, for the given commodity ids, the
	// per-id count of open loans. Used by the list page to drive the
	// "lent out" badge in a single round-trip rather than N+1. Empty
	// input returns an empty map; missing ids map to 0.
	CountOpenByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error)
}

// ServiceState filters CommodityServiceRegistry.ListPaginated. Mirrors
// LoanState — same conventions ("all" sentinel, empty string treated as
// "all" by callers, IsValid returns false on "" so handlers fall back
// explicitly). The names map to the operational meaning of each state:
//
//   - "open": currently with the workshop (returned_at IS NULL)
//   - "overdue": "open" + expected_return_at set + before today
//   - "completed": came back ("returned_at IS NOT NULL"). Named differently
//     from loan's "returned" because workshops "complete" jobs and the FE
//     copy reads more naturally.
type ServiceState string

const (
	ServiceStateAll       ServiceState = "all"
	ServiceStateOpen      ServiceState = "open"
	ServiceStateOverdue   ServiceState = "overdue"
	ServiceStateCompleted ServiceState = "completed"
)

// IsValid reports whether s is one of the known service states. Empty
// string returns false so handlers fall back to ServiceStateAll explicitly.
func (s ServiceState) IsValid() bool {
	switch s {
	case ServiceStateAll, ServiceStateOpen, ServiceStateOverdue, ServiceStateCompleted:
		return true
	}
	return false
}

// ServiceListOptions narrows the result of CommodityServiceRegistry.ListPaginated.
type ServiceListOptions struct {
	// State filters by service state. Empty (or ServiceStateAll) returns all.
	State ServiceState
	// Now is the server clock used to evaluate ServiceStateOverdue. Pass
	// time.Time{} for "use real now"; tests pass a frozen value.
	Now time.Time
}

// CommodityServiceRegistry is the group-scoped registry of commodity_services.
// Mirrors CommodityLoanRegistry one-to-one — the only differences are
// the field names ("sent_at" vs "lent_at", "expected_return_at" vs
// "due_back_at") and the cost columns the loan table doesn't carry. See
// the type-level comment on CommodityLoanRegistry for the cross-cutting
// design notes (single open row, no DB-level uniqueness, etc.).
type CommodityServiceRegistry interface {
	Registry[models.CommodityService]

	// ListByCommodity returns all service rows (open + completed) for a
	// single commodity, ordered most-recent-first. Used by the per-item
	// Service tab.
	ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityService, error)

	// GetOpenForCommodity returns the (at most one) open service row for
	// a commodity, or registry.ErrNotFound if there isn't one.
	GetOpenForCommodity(ctx context.Context, commodityID string) (*models.CommodityService, error)

	// ListPaginated returns a paginated, group-wide list of services
	// filtered by state, ordered by sent_at desc. Pass a zero
	// ServiceListOptions for "all rows".
	ListPaginated(ctx context.Context, offset, limit int, opts ServiceListOptions) ([]*models.CommodityService, int, error)

	// CountOpenByCommodity returns, for the given commodity ids, the
	// per-id count of open service rows. Drives the "in service" list
	// badge.
	CountOpenByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error)
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

// PreviewTokenInputs is the deterministic, replay-resistant payload
// covered by the HMAC of a currency migration preview token. The
// state-hash captures (commodity_count, sum(current_price)) at preview
// time; if anything in the group changes between preview and commit,
// the recomputed hash differs and the start handler returns
// 409 currency_migration.state_changed.
type PreviewTokenInputs struct {
	GroupID      string
	FromCurrency string
	ToCurrency   string
	Rate         string // canonical decimal string (no scientific notation)
	StateHash    string // hex-encoded SHA-256 over (count || sum_current_price)
	ExpiresAt    time.Time
}

// CurrencyMigrationRegistry manages the long-running per-group currency
// migration rows introduced in #1550 (epic #202). Mirrors
// RestoreOperationRegistry's two-tx lifecycle. All worker-side methods
// (ClaimNextPending, SweepStuckRunning, WriteAuditRow) require the
// service registry (background-worker RLS bypass); the user-facing
// Create / Get / List path goes through the user registry.
type CurrencyMigrationRegistry interface {
	Registry[models.CurrencyMigration]

	// LatestForGroup returns the most-recently created migration row for
	// the current group, or ErrNotFound when the group has never been
	// migrated. Used by the FE settings panel to show the last attempt.
	LatestForGroup(ctx context.Context, groupID string) (*models.CurrencyMigration, error)

	// InFlightForGroup returns the (at most one) pending|running row for
	// the group, or (nil, nil) if none. Used by the start handler's
	// pre-insert check and by the lock middleware to surface 423.
	InFlightForGroup(ctx context.Context, groupID string) (*models.CurrencyMigration, error)

	// CompletedTodayForGroup counts the group's completed migrations
	// since UTC midnight on `now`. Backs the daily-cap enforcement
	// (currencyMigrationDailyCap = 2) at the start endpoint.
	CompletedTodayForGroup(ctx context.Context, groupID string, now time.Time) (int, error)

	// UpdateStatus mutates only (status, started_at, completed_at,
	// error_message, commodity_count, total_before, total_after) on the
	// row identified by id. The worker uses this for the TX1 flip and
	// the TX2 final write. Other columns stay frozen.
	UpdateStatus(ctx context.Context, id string, patch CurrencyMigrationStatusPatch) error

	// WriteAuditRow inserts a single per-commodity audit image. The worker
	// calls this once per commodity inside TX2.
	WriteAuditRow(ctx context.Context, row models.CurrencyMigrationAuditRow) (*models.CurrencyMigrationAuditRow, error)

	// ListAuditRows returns every audit row for a migration in stable
	// (created_at, id) order. Drives the "what changed" history view.
	ListAuditRows(ctx context.Context, migrationID string) ([]*models.CurrencyMigrationAuditRow, error)

	// ClaimNextPending atomically picks one pending row, flips it to
	// running (TX1) and returns it. Uses SELECT FOR UPDATE SKIP LOCKED
	// in postgres so multiple workers don't collide. Returns
	// (nil, ErrNotFound) when no pending work exists.
	ClaimNextPending(ctx context.Context) (*models.CurrencyMigration, error)

	// SweepStuckRunning flips every running row with started_at older
	// than now-threshold to failed (with a generic error message),
	// clearing the matching location_groups.currency_migration_id. The
	// background worker calls this every tick AND on startup. Returns
	// the rows it transitioned.
	SweepStuckRunning(ctx context.Context, now time.Time, threshold time.Duration) ([]*models.CurrencyMigration, error)

	// IssuePreviewToken signs `inputs` with the registry's HMAC key and
	// returns the encoded token. The token is stateless — IssuePreviewToken
	// does not write anything; VerifyPreviewToken re-derives the
	// signature from the same key and compares.
	IssuePreviewToken(inputs PreviewTokenInputs) (string, error)

	// VerifyPreviewToken returns the decoded inputs if the token's
	// signature matches and its expiry has not passed. ErrPreviewTokenInvalid
	// or ErrPreviewTokenExpired otherwise.
	VerifyPreviewToken(token string, now time.Time) (PreviewTokenInputs, error)
}

// CurrencyMigrationStatusPatch is the narrow update payload for
// UpdateStatus — only the worker-managed lifecycle fields. Pointer
// fields are "leave alone if nil, write if non-nil"; status is the
// always-required new state.
type CurrencyMigrationStatusPatch struct {
	Status         models.CurrencyMigrationStatus
	StartedAt      *time.Time
	CompletedAt    *time.Time
	ErrorMessage   *string
	CommodityCount *int
	TotalBefore    *string // canonical decimal text; pointer-to-string keeps "no change" distinct from "set to 0"
	TotalAfter     *string
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

// WarrantyReminderRegistry is the worker-only registry that records
// "reminder X for commodity Y at threshold Z has been emitted" rows.
// The (commodity_id, threshold_days) tuple is unique — Create returns
// (false, nil) for the loser of a race so the worker can treat the
// happy path and the race-loser path identically (both mean "no email
// is needed from this tick").
//
// All operations run under the background-worker RLS bypass. There is
// no user-facing surface on this table.
type WarrantyReminderRegistry interface {
	// HasSent reports whether a reminder row already exists for the
	// given (commodity, threshold) tuple. Used by the worker to skip
	// the email-render path when the row is present.
	HasSent(ctx context.Context, commodityID string, thresholdDays int) (bool, error)

	// CreateOnce attempts to insert the reminder row. Returns
	// (true, nil) if this call won the insert and the caller may
	// proceed to send the email. Returns (false, nil) when a row for
	// the same tuple already exists (idempotency: another tick or
	// process beat us). Other errors are returned as-is.
	CreateOnce(ctx context.Context, reminder models.WarrantyReminder) (bool, error)
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

	// CountByUser returns the number of memberships a user holds in
	// the given tenant. Used by the per-user membership cap check
	// (#1388) — a SELECT COUNT(*) avoids materializing the rows when
	// only the size matters.
	CountByUser(ctx context.Context, tenantID, userID string) (int, error)

	// CountAdminsByGroup returns the number of role>=admin (admin or
	// owner) memberships in a group. Renamed-semantics rather than
	// strictly counting `role = 'admin'` rows: every owner is an admin
	// in capability, and the call sites that use this method (mostly the
	// last-admin guard before the role-taxonomy expansion) were always
	// asking "is anyone still capable of admin?". For the stricter
	// "≥1 owner per group" invariant, use CountOwnersByGroup instead.
	CountAdminsByGroup(ctx context.Context, groupID string) (int, error)

	// CountOwnersByGroup returns the number of memberships with role
	// = 'owner' in a group. Used by RemoveMember / UpdateMemberRole to
	// enforce the post-#1533 invariant that every group must always
	// have at least one owner (since only owners can delete the group).
	CountOwnersByGroup(ctx context.Context, groupID string) (int, error)

	// CountByGroup returns the total number of memberships in a group.
	// A row in group_memberships represents an accepted membership;
	// pending invites live in group_invites and are intentionally not
	// included. Used to surface members_count on the LocationGroup
	// JSON:API resource (#1650) without forcing the FE to fetch the
	// full members list just to render a count.
	CountByGroup(ctx context.Context, groupID string) (int, error)

	// CountByGroups returns membership counts for several groups in
	// one round-trip. Used by GET /groups so the listing handler can
	// enrich every LocationGroup with members_count with a single
	// extra query instead of N. The returned map keys every input
	// group ID (zero when no memberships exist) so callers don't have
	// to handle the missing-key case.
	CountByGroups(ctx context.Context, groupIDs []string) (map[string]int, error)

	// ListByGroupWithUsers returns every membership for a group joined
	// with its User (id, email, name). Used by the members list
	// endpoint to ship the data the UI needs in a single round-trip
	// instead of a follow-up users:included fetch per row.
	ListByGroupWithUsers(ctx context.Context, groupID string) ([]*models.MembershipWithUser, error)

	// CreateUnderCap mints a membership only if the target user holds
	// fewer than maxMemberships rows in the same tenant. The check
	// and the insert run inside one transaction with a per-(tenant,
	// user) advisory lock so two concurrent CreateGroup / AddMember /
	// AcceptInvite calls can't both pass a stale check and exceed the
	// cap. Returns (nil, true, nil) when the user is already at or
	// over the cap; (nil, false, err) on registry / tx errors.
	CreateUnderCap(ctx context.Context, membership models.GroupMembership, maxMemberships int) (*models.GroupMembership, bool, error)

	// DeleteWithMemberInvariants atomically removes the named
	// membership row while two invariants are enforced inside the
	// same transaction (#1652, defense-in-depth):
	//
	//   A) ≥1 owner   — if the row's role is owner and removing it
	//      would drop the group's owner count to zero, the registry
	//      returns ErrLastOwner without touching the row.
	//   B) ≥1 member  — if removing the row would drop the group's
	//      total membership count to zero (regardless of role), the
	//      registry returns ErrLastMember. Catches the case where
	//      role data has drifted so the owner check passes vacuously
	//      (e.g. the sole member happens to be a `user`).
	//
	// Implementations must take a per-group transactional lock around
	// the count(*) checks and the DELETE so two concurrent leaves on
	// a two-member group can't both pass the count check and both
	// commit the delete. Postgres uses pg_advisory_xact_lock; memory
	// holds the registry write lock for the duration of the
	// count+delete sequence. Returns ErrNotFound when no membership
	// with the given id exists. This is the canonical removal path
	// for `LeaveGroup` and the admin-initiated remove-member API;
	// callers that bypass the invariants (e.g. group deletion) keep
	// using the plain `Delete`.
	DeleteWithMemberInvariants(ctx context.Context, membershipID string) error

	// UpdateRoleWithMemberInvariants atomically swaps the row's role
	// while sharing the same per-group lock as DeleteWithMemberInvariants
	// (#1652). Without this, a concurrent leave + owner-demotion pair
	// can both observe ownerCount=2 before either commits, then both
	// commit, leaving the group with zero owners (the bug
	// DeleteWithMemberInvariants alone can't prevent because the
	// demote path acquired its own lock under the plain `Update`).
	// The two operations now serialize: whichever runs second sees
	// the post-first-op state under the same advisory key.
	//
	// Returns ErrLastOwner when the current role is owner, the new
	// role is not, and the row is the only owner in the group.
	// Returns ErrNotFound when no membership with the given id
	// exists. The updated membership row is returned so the handler
	// can echo it back to the client.
	UpdateRoleWithMemberInvariants(ctx context.Context, membershipID string, newRole models.GroupRole) (*models.GroupMembership, error)
}

// GroupNotificationPrefRegistry stores per-user per-group opt-outs for
// notification categories (issue #1648). A row's presence flips the
// per-group override on; absence falls through to the user-global pref
// from #1373 — see notifications.Service.IsEnabledForGroup for the
// resolution chain.
type GroupNotificationPrefRegistry interface {
	Registry[models.GroupNotificationPref]

	// ListByUserGroup returns every category override for one user
	// inside one group. The unique index on (tenant, group, user,
	// category) guarantees at most one row per category. The result
	// is what the GET /g/<slug>/notifications endpoint reads, and
	// what the warranty worker's per-sweep cache materialises.
	ListByUserGroup(ctx context.Context, tenantID, groupID, userID string) ([]*models.GroupNotificationPref, error)

	// Upsert inserts a new (tenant, group, user, category) row or
	// updates the `enabled` flag on the existing one. Returns the
	// post-write row so callers can echo the saved value back to the
	// client without a follow-up SELECT.
	Upsert(ctx context.Context, pref models.GroupNotificationPref) (*models.GroupNotificationPref, error)
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

	// ListActiveByUserID returns all refresh tokens for a user that have
	// not been revoked and have not yet expired. Used by the sessions
	// endpoint (#1378) — the FE only wants live sessions, never the
	// historical revoked/expired carcasses that the retention sweep
	// will eventually clean up. Ordered most-recently-used first
	// (LastUsedAt desc, CreatedAt desc as the tiebreaker for tokens
	// that have never been used).
	ListActiveByUserID(ctx context.Context, userID string) ([]*models.RefreshToken, error)

	// RevokeByUserID marks all refresh tokens for a user as revoked
	RevokeByUserID(ctx context.Context, userID string) error

	// RevokeByID atomically revokes a single refresh token by id but
	// only if it belongs to the supplied user. Returns ErrNotFound
	// when no row matches the (id, user_id) pair so a user can't
	// revoke someone else's session via a guessed id.
	RevokeByID(ctx context.Context, userID, id string) error

	// RevokeAllExceptID marks every refresh token for a user as revoked
	// except the one whose id matches keepID. Used by the "Sign out all
	// other sessions" button (#1378). Pass an empty keepID to revoke
	// every token — equivalent to RevokeByUserID but kept distinct so
	// the call site reads obvious.
	RevokeAllExceptID(ctx context.Context, userID, keepID string) error

	// DeleteExpired removes all expired refresh tokens from the store
	DeleteExpired(ctx context.Context) error
}

// LoginEventRegistry stores the append-only login_events audit trail
// (issue #1379). The registry runs under the background-worker role so
// the unauthenticated login flow (where no tenant context is set in the
// DB session yet) can still insert rows — this bypasses the
// tenant-isolation RLS policy on reads too, so every read method takes
// an explicit `tenantID` and the SQL adds `tenant_id = $tenantID` as
// defense-in-depth alongside the user_id filter. Without that, a bug in
// a caller could leak login events across tenants even though the row
// has a tenant_id column populated correctly.
type LoginEventRegistry interface {
	Registry[models.LoginEvent]

	// ListByUser returns the most recent login events for the user,
	// newest first, capped at limit. Limit <= 0 falls back to 100.
	// tenantID is required — empty input yields an empty result.
	ListByUser(ctx context.Context, tenantID, userID string, limit int) ([]*models.LoginEvent, error)

	// CountFailedSince returns the number of failed login_events for
	// the user since `since`. "Failed" = outcome != ok. Drives the
	// "We noticed N failed sign-in attempts" banner (#1379). tenantID
	// is required — empty input yields 0.
	CountFailedSince(ctx context.Context, tenantID, userID string, since time.Time) (int, error)

	// DeleteOlderThan removes login_events whose created_at is before
	// cutoff. Called daily by login_event_retention_worker — the rows
	// are append-only so we don't need any tenant/user qualifier here.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error)
}

// GroupPurger hard-deletes every row whose group_id references the given
// LocationGroup, in a FK-safe order: restore_steps, restore_operations,
// exports, files, commodities, areas, locations and finally group_memberships.
// `files` is purged before `commodities` because file rows reference
// commodities polymorphically via (linked_entity_type, linked_entity_id) —
// dropping commodities first leaves orphan rows visible to RLS-bypass queries.
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
	CommodityEventRegistry         CommodityEventRegistry
	SettingsRegistry               SettingsRegistry
	ExportRegistry                 ExportRegistry
	RestoreOperationRegistry       RestoreOperationRegistry
	RestoreStepRegistry            RestoreStepRegistry
	FileRegistry                   FileRegistry
	TagRegistry                    TagRegistry
	CommodityLoanRegistry          CommodityLoanRegistry
	CommodityServiceRegistry       CommodityServiceRegistry
	ThumbnailGenerationJobRegistry ThumbnailGenerationJobRegistry
	UserConcurrencySlotRegistry    UserConcurrencySlotRegistry
	OperationSlotRegistry          OperationSlotRegistry
	TenantRegistry                 TenantRegistry
	UserRegistry                   UserRegistry
	RefreshTokenRegistry           RefreshTokenRegistry
	LoginEventRegistry             LoginEventRegistry            // Append-only login attempt audit (#1379); written by login flow + retention worker
	AuditLogRegistry               AuditLogRegistry              // AuditLogRegistry doesn't need factory as it's not user-aware
	EmailVerificationRegistry      EmailVerificationRegistry     // EmailVerificationRegistry doesn't need factory as it's not user-aware
	PasswordResetRegistry          PasswordResetRegistry         // PasswordResetRegistry doesn't need factory as it's not user-aware
	LocationGroupRegistry          LocationGroupRegistry         // LocationGroupRegistry is tenant-scoped, not user-aware
	GroupMembershipRegistry        GroupMembershipRegistry       // GroupMembershipRegistry is tenant-scoped, not user-aware
	GroupInviteRegistry            GroupInviteRegistry           // GroupInviteRegistry is tenant-scoped, not user-aware
	GroupInviteAuditRegistry       GroupInviteAuditRegistry      // GroupInviteAuditRegistry is tenant-scoped, not user-aware; written only by the group purge worker
	GroupNotificationPrefRegistry  GroupNotificationPrefRegistry // Per-user per-group notification opt-outs (#1648); tenant-scoped, user-filtered in application logic
	GroupPurger                    GroupPurger                   // GroupPurger bulk-removes group-scoped entities during the purge worker's tick
	WarrantyReminderRegistry       WarrantyReminderRegistry      // WarrantyReminderRegistry is the idempotency store for the warranty reminder worker; service-mode only
	CurrencyMigrationRegistry      CurrencyMigrationRegistry     // Currency migration operation rows + audit + HMAC token signing (issue #1550 / epic #202)
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
		validation.Field(&s.CommodityEventRegistry, validation.Required),
		validation.Field(&s.SettingsRegistry, validation.Required),
		validation.Field(&s.ExportRegistry, validation.Required),
		validation.Field(&s.FileRegistry, validation.Required),
		validation.Field(&s.TagRegistry, validation.Required),
		validation.Field(&s.CommodityLoanRegistry, validation.Required),
		validation.Field(&s.CommodityServiceRegistry, validation.Required),
		validation.Field(&s.TenantRegistry, validation.Required),
		validation.Field(&s.UserRegistry, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, s, fields...)
}
