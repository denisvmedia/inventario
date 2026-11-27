package registry

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
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

// AreaRegistry stores areas. Delete refuses an area that still holds live
// items (ErrCannotDelete) and, in the same transaction, clears the area of the
// trashed items filed under it, so Trash never blocks deleting an area; those
// items come back unassigned when restored.
type AreaRegistry interface {
	Registry[models.Area]

	// GetCommodities returns the ids of the live items filed under the area.
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
	// disposed, written_off, donated, stolen). Empty = unrestricted.
	Statuses []models.CommodityStatus
	// AreaID, when non-empty, restricts to a single area. Use "" to
	// disable the filter (rather than a sentinel like "*").
	AreaID string
	// Unassigned, when true and AreaID is empty, restricts the result to
	// commodities that have no area (area_id IS NULL) — the "unassigned"
	// bucket (issue #1986). Ignored when AreaID is set: an explicit area
	// filter always wins, so passing both is the same as filtering by
	// AreaID alone.
	Unassigned bool
	// SellerIDs, when non-empty, restricts the result to commodities whose
	// seller is one of the listed IDs (#2804). Empty = unrestricted.
	SellerIDs []string
	// Search is the free-text query. SplitSearchTerms splits it into terms
	// and a commodity matches when every term matches one of its search
	// fields, its tags or its asset ID — MatchCommoditySearch states the
	// rule. Empty = no search.
	Search string
	// SearchTagLabels maps each term of Search to the slugs of the
	// commodity tags in the group whose label contains it, so a term can
	// find an item by the label of its tag as well as by the slug.
	// services/commodityquery fills it from TagRegistry.SlugsByLabel; a
	// term without an entry matches tags by slug only.
	SearchTagLabels map[string][]string
	// SearchCustomFields says which custom field values a search term is
	// matched against (#2803); see CustomFieldSearch. The zero value reads
	// none.
	SearchCustomFields CustomFieldSearch
	// Tags narrows by the tags items carry; see CommodityTagFilter.
	Tags CommodityTagFilter
	// CustomFields keeps the items that pass every filter in the list
	// (#2803); see CustomFieldFilter.
	CustomFields []CustomFieldFilter
	// CustomFieldSort, when set, orders by a custom field instead of
	// SortField, in the direction of SortDesc; see CustomFieldSort.
	CustomFieldSort *CustomFieldSort
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
	// LentOut, when non-nil, restricts results by whether the commodity
	// has any open loan (commodity_loans row with `returned_at IS NULL`).
	// true = currently lent only; false = currently not-lent only. nil =
	// no filter. Postgres applies this as an EXISTS subquery against
	// commodity_loans; memory walks OpenLoanCommodityIDs.
	LentOut *bool
	// OpenLoanCommodityIDs is the pre-resolved set of commodity IDs in
	// the current group that have at least one open loan. The memory
	// backend uses this to evaluate LentOut without depending on the
	// loan registry. Postgres ignores it (the EXISTS subquery resolves
	// the relationship in-database). Callers (the apiserver handler)
	// populate this iff LentOut is non-nil.
	OpenLoanCommodityIDs []string
	// Trashed selects the items in Trash instead of the live ones; every
	// other field filters them the same way. The loans of a trashed item
	// are hidden, so a caller does not combine it with LentOut
	// (services/commodityquery refuses the pair).
	Trashed bool
	// DraftsOnly keeps only the items marked as draft (Commodity.Draft),
	// whatever their status. The default view hides drafts, so a caller
	// sets IncludeInactive with it; without it nothing matches. It is not
	// part of the item query contract: the inbox reads it (#2794).
	DraftsOnly bool
	// NoPhoto keeps the items without a photo: no live file in the item's
	// images bucket with Type=image and Category=images, the files
	// FirstPhotos picks a cover from. Postgres applies it as a NOT EXISTS
	// subquery on the file links; memory reads PhotoCommodityIDs. The files
	// of an item in Trash are in Trash too, so a caller does not combine it
	// with Trashed (services/commodityquery refuses the pair).
	NoPhoto bool
	// PhotoCommodityIDs is the pre-resolved set of the group's item IDs
	// that have a photo, for a backend that is not a NativePhotoFilterer.
	// Callers populate it iff NoPhoto is set; postgres ignores it.
	PhotoCommodityIDs []string
	// NoPurchasePrice keeps the items whose purchase price (OriginalPrice)
	// is unknown. A known zero is a price and does not match (#2813).
	NoPurchasePrice bool
	// NoPurchaseDate keeps the items whose purchase date is unknown.
	NoPurchaseDate bool
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

// restoreAcquisitionCtxKey keys a trusted, restore-only acquisition pair on a
// context (see WithRestoreAcquisition).
type restoreAcquisitionCtxKey struct{}

type restoreAcquisition struct {
	price    decimal.Decimal
	currency models.Currency
}

// WithRestoreAcquisition marks ctx so the next CommodityRegistry.Create writes
// the given write-once acquisition provenance pair (acquisition_price /
// acquisition_currency, epic #202) onto the freshly created row instead of
// clearing it. This is the single trusted seam by which the signature-verified
// #534 backup restore reconstructs a commodity's archived acquisition history.
//
// It is deliberately a context signal, NOT a CommodityRegistry method, so the
// write-once bypass is not exposed on the registry surface: the normal API path
// never sets it, so Create still clears acquisition and Update still preserves
// the existing DB values — acquisition stays server-managed and immutable for
// every user write.
func WithRestoreAcquisition(ctx context.Context, price decimal.Decimal, currency models.Currency) context.Context {
	return context.WithValue(ctx, restoreAcquisitionCtxKey{}, restoreAcquisition{price: price, currency: currency})
}

// RestoreAcquisitionFromContext returns the trusted restore acquisition pair set
// by WithRestoreAcquisition, if present. Consumed only by the CommodityRegistry
// Create implementations.
func RestoreAcquisitionFromContext(ctx context.Context) (price decimal.Decimal, currency models.Currency, ok bool) {
	v, vok := ctx.Value(restoreAcquisitionCtxKey{}).(restoreAcquisition)
	if !vok {
		return decimal.Decimal{}, "", false
	}
	return v.price, v.currency, true
}

// restoreAssetNumberCtxKey keys a trusted, restore-only asset number on a
// context (see WithRestoreAssetNumber).
type restoreAssetNumberCtxKey struct{}

// WithRestoreAssetNumber marks ctx so the next CommodityRegistry.Create keeps
// the archived asset number n instead of assigning the next free one, as long
// as no other commodity in the target group holds n already. When n is taken
// the registry assigns the next free number, so a restore never produces two
// items with the same asset ID. It is the backup restore's seam, a context
// signal rather than a registry method for the same reason as
// WithRestoreAcquisition: the API path never sets it, so a user cannot pick
// an asset number.
func WithRestoreAssetNumber(ctx context.Context, n int) context.Context {
	return context.WithValue(ctx, restoreAssetNumberCtxKey{}, n)
}

// RestoreAssetNumberFromContext returns the archived asset number set by
// WithRestoreAssetNumber, if present and positive. Consumed only by the
// CommodityRegistry Create implementations.
func RestoreAssetNumberFromContext(ctx context.Context) (int, bool) {
	n, ok := ctx.Value(restoreAssetNumberCtxKey{}).(int)
	if !ok || n <= 0 {
		return 0, false
	}
	return n, true
}

// restoredDraftCtxKey keys the archived state of a draft that a backup
// restore recreates (see WithRestoredDraft).
type restoredDraftCtxKey struct{}

// RestoredDraft is the archived revision and timestamps of a draft.
type RestoredDraft struct {
	Revision  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WithRestoredDraft marks ctx so a CommodityDraftRegistry.Create or
// CreateOrGet that creates a draft keeps the archived revision and timestamps
// instead of starting at revision 1 now, so a restored draft keeps its place
// in the drafts list. It is the backup restore's seam, a context signal for
// the same reason as WithRestoreAssetNumber: the API path never sets it.
func WithRestoredDraft(ctx context.Context, d RestoredDraft) context.Context {
	return context.WithValue(ctx, restoredDraftCtxKey{}, d)
}

// RestoredDraftFromContext returns the state set by WithRestoredDraft. A
// revision below 1 or a zero timestamp is replaced by what a new draft gets:
// revision 1 and now.
func RestoredDraftFromContext(ctx context.Context, now time.Time) (RestoredDraft, bool) {
	d, ok := ctx.Value(restoredDraftCtxKey{}).(RestoredDraft)
	if !ok {
		return RestoredDraft{}, false
	}
	if d.Revision < 1 {
		d.Revision = 1
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = d.CreatedAt
	}
	return d, true
}

// CommodityName is an item's ID and name, and whether it is in Trash.
type CommodityName struct {
	ID      string `db:"id"`
	Name    string `db:"name"`
	Trashed bool   `db:"trashed"`
}

// CommodityRegistry stores items.
//
// Trash: an item moved to Trash keeps its row with deleted_at set
// (models.Commodity.IsTrashed). Every read of this interface returns live items
// only — Get, List, Count, ListPaginated, GetMany — so a trashed item is absent
// from lists, counts, search, reports, exports and reminders without each
// caller filtering for it. The exceptions are named: ListByGroup and
// BackfillAssetNumbers see trashed items too, and the *Trashed methods and
// ListPaginated with CommodityListOptions.Trashed see trashed items only. Update refuses a trashed item with ErrNotFound. Code that
// queries the commodities table directly (a new worker, a report) has to apply
// the same filter: in SQL, `deleted_at IS NULL`; in Go, Commodity.IsTrashed.
type CommodityRegistry interface {
	Registry[models.Commodity]

	// GetTrashed returns a trashed item by id; ErrNotFound when it does not
	// exist or is live.
	GetTrashed(ctx context.Context, id string) (*models.Commodity, error)

	// ListTrashed returns a page of the group's trashed items, most recently
	// deleted first, with the total.
	ListTrashed(ctx context.Context, offset, limit int) ([]*models.Commodity, int, error)

	// ListTrashedBefore returns up to limit trashed items, in every tenant and
	// group, that were deleted before cutoff, oldest first. Service mode only:
	// it backs the purge of expired Trash.
	ListTrashedBefore(ctx context.Context, cutoff time.Time, limit int) ([]*models.Commodity, error)

	// Trash moves a live item to Trash: it stamps deleted_at with at,
	// deleted_by with deletedBy and remembers the item's area. ErrNotFound when
	// the item does not exist or is already in Trash.
	Trash(ctx context.Context, id, deletedBy string, at time.Time) (*models.Commodity, error)

	// Restore makes a trashed item live again and returns it. ErrNotFound when
	// the item does not exist or is live.
	Restore(ctx context.Context, id string) (*models.Commodity, error)

	// SetDraft sets or clears the draft mark of a live item and writes no
	// other column. It reads and writes the item under the item's row lock,
	// so it and an Update of the same item run one after the other and
	// neither undoes the other's change. check, when not nil, receives the
	// item as it will be stored, under that lock; an error from it is
	// returned as is and nothing is written. It returns the item as it was
	// under the lock and as it is stored after the write, so a caller that
	// records what changed compares the two and not an earlier read.
	// ErrNotFound when the item does not exist or is in Trash.
	SetDraft(ctx context.Context, id string, draft bool, check func(models.Commodity) error) (before, after *models.Commodity, err error)

	// Patch changes the columns a bulk action edits on a live item: its
	// tags, type, status, status date, status note, sale price and area. It
	// reads the item under the item's row lock, hands a copy to change, and
	// writes those columns of the item change returns. Every other column
	// and the custom field values keep what is stored, so Patch never writes
	// back a value an edit saved since the caller last read the item.
	//
	// addTags names every tag change may add. They are locked before the
	// item, as every write of an item's tags locks them, and a change that
	// adds any other tag fails with ErrInvalidInput. A new type must be one
	// of the group's item types (ErrUnknownItemType), and a new area one the
	// group can see (ErrNotFound).
	//
	// change reports whether it changed anything; when it did not, nothing
	// is written and after is nil. An error from change is returned as is,
	// and nothing is written. before is the item as it was under the lock
	// and after the item as stored. ErrNotFound when the item does not exist
	// or is in Trash.
	Patch(ctx context.Context, id string, addTags []string, change func(models.Commodity) (models.Commodity, bool, error)) (before, after *models.Commodity, err error)

	// PurgeTrashed deletes the row of a trashed item; its loans, service
	// records, maintenance schedules, supply links and events go with it
	// through ON DELETE CASCADE. It never deletes a live item, so a purge
	// racing a restore loses. Reports whether a row was deleted. Like Delete,
	// it leaves the item's file links alone: purging goes through
	// services.TrashService.
	PurgeTrashed(ctx context.Context, id string) (bool, error)

	// BackfillAssetNumbers gives every commodity that has no asset number
	// one, group by group, continuing after the group's highest number in
	// creation order (earliest history event, then registered date, then
	// id). It takes the same per-group lock as Create, so it is safe to run
	// while the app serves requests, and it is idempotent: once every row is
	// numbered it changes nothing. Returns how many rows it numbered. Meant
	// for the service registry; a user registry only sees its own group.
	BackfillAssetNumbers(ctx context.Context) (int, error)

	// BackfillLegacyPrices rewrites the rows stored before a price could be
	// unknown (models.Commodity.LegacyZeroPrices): a zero current price or
	// converted price, which then meant "not set", becomes unknown, and the
	// flag is cleared. The purchase price keeps its stored value, because a
	// zero there may be a real free acquisition and nothing tells the two
	// apart. It is idempotent and safe to run while the app serves requests:
	// rows written since the flag existed are never touched. Returns how many
	// rows it rewrote. Meant for the service registry.
	BackfillLegacyPrices(ctx context.Context) (int, error)

	// ListPaginated returns a paginated list of commodities along with the total count,
	// optionally filtered and sorted via opts. Pass a zero CommodityListOptions for the
	// previous "all rows, name+id ascending" behaviour.
	ListPaginated(ctx context.Context, offset, limit int, opts CommodityListOptions) ([]*models.Commodity, int, error)

	// CountTags counts, among the commodities opts selects, the items that
	// carry each tag and the items without tags. Sorting and pagination do
	// not apply.
	CountTags(ctx context.Context, opts CommodityListOptions) (CommodityTagCounts, error)

	// CountEach counts, for each entry of opts, the commodities it selects,
	// as ListPaginated would report in its total, without reading the rows.
	// The counts are read from one snapshot, so they describe one moment.
	CountEach(ctx context.Context, opts []CommodityListOptions) ([]int, error)

	// ListBrands returns the brands the commodities of the group carry, with
	// how many carry each, for the brand input's suggestions. Every live
	// commodity counts, drafts and inactive items included; items in Trash
	// do not. A brand matches
	// when it contains query as a case-insensitive literal substring; an
	// empty query matches every brand. CountCommodityBrands states the rules
	// and the order; at most limit brands are returned.
	ListBrands(ctx context.Context, query string, limit int) ([]CommodityBrandCount, error)

	// ListIDs returns the ids of at most limit commodities opts selects, in
	// no particular order. It reads ids only, so a caller can tell which
	// items a query selects without loading them. Sorting does not apply.
	ListIDs(ctx context.Context, opts CommodityListOptions, limit int) ([]string, error)

	// ListByGroup returns every commodity in the given (tenant_id, group_id)
	// tuple, regardless of draft / status, trashed items included. Used by the
	// currency-migration service (issue #202): the conversion needs to see all
	// rows in the group, not just the user-visible "in_use, non-draft" subset
	// that ListPaginated defaults to, and a trashed item restored later must
	// carry prices in the group's current currency. Service-mode callers (the worker) bypass
	// RLS via this; user-mode callers should use the registry's group
	// context instead.
	ListByGroup(ctx context.Context, tenantID, groupID string) ([]*models.Commodity, error)

	// GetMany fetches a batch of commodities by id in a single round-trip.
	// The returned order is unspecified — callers that need positional
	// alignment with `ids` must build their own id→commodity map. IDs not
	// present in the caller's RLS-visible scope (cross-tenant rows, deleted
	// rows) are silently dropped rather than surfacing as errors, since the
	// FK is ON DELETE CASCADE: a row either still exists in scope, or has
	// been cascaded away. An empty `ids` slice returns (nil, nil) without
	// touching the storage layer; duplicate ids in the slice are collapsed
	// (the result lists each commodity at most once). Backs the batched
	// commodity fetch in listGroupLoans (issue #1512); the cover-resolver
	// follow-up (#1451) is expected to reuse the same primitive.
	GetMany(ctx context.Context, ids []string) ([]*models.Commodity, error)

	// GetNames returns the name of each item among ids, live or in Trash,
	// and whether it is in Trash, in one round trip and in unspecified
	// order. An id with no item in scope is dropped, and a repeated id is
	// listed once. It names the items that item link values point at.
	GetNames(ctx context.Context, ids []string) ([]CommodityName, error)

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

// NativeLentOutFilterer is the capability marker implemented by
// CommodityRegistry backends that resolve CommodityListOptions.LentOut
// via a single-query database join (the postgres backend does this with
// an `EXISTS` subquery on commodity_loans). Callers (apiserver) use a
// type assertion against this interface to decide whether to pre-resolve
// OpenLoanCommodityIDs from CommodityLoanRegistry — backends without
// the capability (memory) need the pre-resolved set; backends that
// implement it can ignore the slice and keep the request to one query.
//
// The method is a no-op marker; the type assertion is the actual gate.
type NativeLentOutFilterer interface {
	// SupportsNativeLentOutFilter is a no-op marker. Implementations
	// signal capability by defining the method; callers only ever
	// check the type assertion, never call the method directly.
	SupportsNativeLentOutFilter()
}

// NativePhotoFilterer is the capability marker of a CommodityRegistry that
// resolves CommodityListOptions.NoPhoto in the database, as postgres does
// with a NOT EXISTS subquery on the file links. Callers pre-resolve
// PhotoCommodityIDs from FileRegistry.CommodityIDsWithPhotos only for a
// backend without it. As with NativeLentOutFilterer, the type assertion is
// the gate and the method is never called.
type NativePhotoFilterer interface {
	SupportsNativePhotoFilter()
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

	// ClaimPending moves an export from pending to in_progress and reports
	// whether this caller is the one that moved it. The condition lives in
	// the write, so two workers racing for the same row get one true and one
	// false instead of both producing an artifact (#2472).
	ClaimPending(ctx context.Context, id string) (bool, error)
}

// FileRegistry stores files and their links.
//
// Trash: a file moved to Trash keeps its row, links and blob with deleted_at
// set (models.FileEntity.IsTrashed). The ordinary reads return live files
// only — Get, List, Count, ListByType, ListByLinkedEntity,
// ListByLinkedEntityAndMeta, ListByChecksum, Search, ListPaginated and
// CountByCategory. The reads that guard bytes or sweep whole groups see
// trashed files too, because a trashed file still owns its blob:
// ListByGroup, ListByTenant, ListPendingChecksum, ListPendingSizeBackfill,
// ExistingIDs and ListIDsByOriginalPath. ListOrphanCandidates returns live
// files only: a trashed file is the Trash purge's to settle, and a link to a
// trashed item counts as a link to an existing entity. The storage sums count
// trashed bytes in their own Trash bucket. The *Trashed methods
// see trashed files only. Update refuses a trashed file with ErrNotFound.
type FileRegistry interface {
	Registry[models.FileEntity]

	// GetTrashed returns a trashed file by id; ErrNotFound when it does not
	// exist or is live.
	GetTrashed(ctx context.Context, id string) (*models.FileEntity, error)

	// ListTrashed returns a page of the group's files that were deleted on
	// their own (DeletedWithCommodityID is nil), most recently deleted first,
	// with the total. Files that went to Trash with an item are listed under
	// that item instead.
	ListTrashed(ctx context.Context, offset, limit int) ([]*models.FileEntity, int, error)

	// ListTrashedLinkedTo returns the trashed files linked to the entity,
	// newest first.
	ListTrashedLinkedTo(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error)

	// CountTrashedWith returns, for each item id, how many files went to Trash
	// with it and their total size. Ids without such files are absent.
	CountTrashedWith(ctx context.Context, commodityIDs []string) (map[string]TrashedFilesStats, error)

	// ListTrashedBefore returns up to limit files, in every tenant and group,
	// that were deleted on their own before cutoff. Oldest first. Service mode
	// only: it backs the purge of expired Trash.
	ListTrashedBefore(ctx context.Context, cutoff time.Time, limit int) ([]*models.FileEntity, error)

	// ListStrandedTrashed returns up to limit trashed files that went to
	// Trash with an item that is no longer in Trash: purged, or live again.
	// Such a file is neither a Trash entry of its own nor listed under an
	// item, so nothing shows it; an interrupted purge, restore or delete
	// leaves it behind. services.TrashService settles it: restores it, hands
	// it to another trashed item or deletes it. Oldest first, whatever the
	// age. A user registry reads its group; a service registry reads every
	// group.
	ListStrandedTrashed(ctx context.Context, limit int) ([]*models.FileEntity, error)

	// PatchTags changes the tags of a live file. It reads the file under the
	// file's row lock, hands a copy to change, and writes the tags of the
	// file change returns with a new update time; every other column and
	// the links keep what is stored. addTags, change, before and after work
	// as in CommodityRegistry.Patch. ErrNotFound when the file does not
	// exist or is in Trash.
	PatchTags(ctx context.Context, id string, addTags []string, change func(models.FileEntity) (models.FileEntity, bool, error)) (before, after *models.FileEntity, err error)

	// Trash moves a live file to Trash. withCommodityID names the item whose
	// deletion takes the file along, or is empty for a file deleted on its
	// own. ErrNotFound when the file does not exist or is already in Trash.
	Trash(ctx context.Context, id, deletedBy, withCommodityID string, at time.Time) error

	// TrashUnlessHeld moves a live file to Trash unless a link other than
	// the one to the entity named by entityType and entityID holds it live,
	// and reports whether it did. Every link holds a file live except one to
	// an item in Trash; when such links are all that is left, the file goes
	// to Trash with the item linked first, so restoring that item brings the
	// file back. The check and the update are one statement, so a link added
	// concurrently keeps the file. Discarding a draft calls it for each of
	// the draft's files.
	TrashUnlessHeld(ctx context.Context, id, entityType, entityID, deletedBy string, at time.Time) (bool, error)

	// Restore makes a trashed file live again. ErrNotFound when the file does
	// not exist or is live.
	Restore(ctx context.Context, id string) error

	// SetTrashedWith changes which item a trashed file went to Trash with
	// (empty makes it a Trash entry of its own). Purging an item uses it to
	// hand a shared file to another trashed item that still links it.
	SetTrashedWith(ctx context.Context, id, commodityID string) error

	// PurgeTrashed deletes the row of a trashed file and its links. It never
	// deletes a live file. Like Delete, it does not touch the blob: callers go
	// through services.FileService.PurgeTrashedFile.
	PurgeTrashed(ctx context.Context, id string) (bool, error)

	// ListByType returns files filtered by type
	ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error)

	// ListByLinkedEntity returns the files linked to an entity through
	// file_links, newest first.
	ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error)

	// CountLinkedTo returns, for each of the entities of entityType, how many
	// live files are linked to it. Entities without files are absent.
	CountLinkedTo(ctx context.Context, entityType string, entityIDs []string) (map[string]int, error)

	// ListByLinkedEntityAndMeta returns the files whose link to the entity
	// carries the given meta (bucket), newest first.
	ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, meta string) ([]*models.FileEntity, error)

	// GetMany returns the live files among ids, with their links, in one
	// read. Ids that name no live file are skipped and duplicates are
	// collapsed. An empty ids returns (nil, nil).
	GetMany(ctx context.Context, ids []string) ([]*models.FileEntity, error)

	// FirstPhotos returns, keyed by item id, the oldest photo of each item
	// among commodityIDs that is not in Trash, in one read. A photo is a live
	// file linked in the item's images bucket with Type=image and
	// Category=images; the oldest by created_at wins, the lowest id on a
	// tie. Items without a photo are absent from the map.
	FirstPhotos(ctx context.Context, commodityIDs []string) (map[string]*models.FileEntity, error)

	// CommodityIDsWithPhotos returns the ID of every item of the group that
	// has a photo in the sense of FirstPhotos, live items and items in
	// Trash alike, each once and in no particular order. It backs the
	// NoPhoto filter of a CommodityRegistry that is not a
	// NativePhotoFilterer.
	CommodityIDsWithPhotos(ctx context.Context) ([]string, error)

	// AddLink attaches a file to an entity. FileID, EntityType, EntityID and
	// Meta come from the argument; the registry sets the ids, tenant, group,
	// creator and created_at. Idempotent: when the file is already linked to
	// that entity the stored link is returned with created=false and left
	// unchanged. The caller is responsible for checking that the target exists
	// in the file's group.
	AddLink(ctx context.Context, link models.FileLink) (stored *models.FileLink, created bool, err error)

	// RemoveLink removes one link of a file and returns it. ErrNotFound when
	// the file carries no link with that id. Removing a file's last link
	// leaves a standalone file; the file itself is never deleted here.
	RemoveLink(ctx context.Context, fileID, linkID string) (*models.FileLink, error)

	// RemoveLinksToEntity removes every link to the entity and returns the ids
	// of the files that lost one. The entity-deletion paths call it after the
	// entity row is gone, then delete the files left without links.
	RemoveLinksToEntity(ctx context.Context, entityType, entityID string) ([]string, error)

	// DeleteIfUnlinked deletes the file row only when no link points at it,
	// ignoring the links to the entity named by ignoreEntityType and
	// ignoreEntityID (empty strings ignore nothing); those go with the row.
	// The check and the delete hold the row lock, so a link added concurrently
	// keeps the file. Reports whether the row was deleted. Like Delete, it does
	// not touch the blob: callers go through FileService.DeleteFileIfUnlinked.
	DeleteIfUnlinked(ctx context.Context, id, ignoreEntityType, ignoreEntityID string) (bool, error)

	// ListByChecksum returns the files whose SHA-256 equals sha256, newest
	// first. An empty checksum matches nothing.
	ListByChecksum(ctx context.Context, sha256 string) ([]*models.FileEntity, error)

	// ListPendingChecksum returns up to limit files that have a blob but no
	// checksum yet, ordered by id and starting after afterID, for the startup
	// checksum pass. Service mode only.
	ListPendingChecksum(ctx context.Context, afterID string, limit int) ([]*models.FileEntity, error)

	// SetChecksum stores a file's SHA-256 when it has none. It writes only that
	// column, so it cannot overwrite a concurrent metadata edit. Service mode
	// only.
	SetChecksum(ctx context.Context, id, sha256 string) error

	// MoveLegacyLinks moves the single link that up to limit files still carry
	// in their legacy linked_entity_* columns into file_links, and empties those
	// columns in the same transaction, so a row is moved exactly once and a
	// rerun is a no-op. Rows whose legacy link has a type but no id carry no
	// usable link and are only emptied. Returns how many rows it processed;
	// zero means nothing is left. Service mode only.
	MoveLegacyLinks(ctx context.Context, limit int) (int, error)

	// ListByGroup returns every file belonging to the given (tenant_id,
	// group_id) tuple, trashed files included. Used by the group purge worker
	// to find physical blobs to delete before the row-level purge wipes the
	// file table — avoids the O(tenant × total_files) scan that List() would
	// perform. Only makes sense for service-mode callers: group-scoped user
	// registries already see exactly the right slice via RLS.
	ListByGroup(ctx context.Context, tenantID, groupID string) ([]*models.FileEntity, error)

	// ListByTenant returns every file of the tenant, trashed files included.
	// Service mode only: the tenant hard delete uses it to find the blobs to
	// delete before the rows go.
	ListByTenant(ctx context.Context, tenantID string) ([]*models.FileEntity, error)

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
	// same filters as Search (text query, file type, tags, and the
	// linked-entity pair). Backs the GET /files/category-counts endpoint
	// that drives the four-tile UI and the cumulative footer on the Files
	// page, and the chip counts on an entity's Files tab — which is what
	// the linked-entity pair is for: counts have to agree with the list
	// beside them, and that list is one entity's files.
	CountByCategory(ctx context.Context, query string, fileType *models.FileType, tags []string, linkedEntityType, linkedEntityID *string) (map[models.FileCategory]int, map[models.FileCategory]int64, error)

	// SumSizeBreakdown returns per-bucket byte totals for the current
	// (tenant, group) scope. Backs GET /g/{slug}/storage-usage (#1388).
	// Export bundles are split out from the FileCategoryOther bucket
	// because they aren't user-facing files in the four-tile UI; the
	// quota visualization lists them as a distinct row.
	SumSizeBreakdown(ctx context.Context) (StorageBreakdown, error)

	// SumSizeBreakdownByGroup mirrors SumSizeBreakdown but for an
	// explicit (tenant_id, group_id) tuple instead of the RLS-scoped
	// caller (#1585). Service-mode only — used by the storage quota
	// warning worker, which iterates every group from a background
	// context where no per-group RLS is active. The same export-bundle
	// split applies.
	SumSizeBreakdownByGroup(ctx context.Context, tenantID, groupID string) (StorageBreakdown, error)

	// ListPendingSizeBackfill streams up to limit file rows whose
	// size_bytes is still zero — the rows that pre-date #1388 and need
	// the boot-time backfill to re-stat the blob and write the actual
	// size. Service-mode only; the backfill runs across every tenant
	// and group, bypassing RLS. Implementations may return fewer rows
	// than limit (the queue is exhausted) but must never return more.
	ListPendingSizeBackfill(ctx context.Context, limit int) ([]*models.FileEntity, error)

	// ListOrphanCandidates streams up to limit file rows that *may* be
	// orphans — files whose every link points at an entity that no longer
	// exists (#2237). It is the discovery half of the orphan-file GC; the
	// worker independently re-verifies every candidate before deleting
	// anything, so this is a filter, never an authorization to delete.
	//
	// The predicate implementations MUST honour, exactly:
	//
	//   1. The file has at least one link, and every link's entity_type is
	//      in ('commodity', 'area', 'location') — a positive ALLOWLIST,
	//      never a negation. A file with no link is a legitimate
	//      STANDALONE file (#2235: first-class, exported in backups) and an
	//      'export' link belongs to the backup subsystem's own lifecycle;
	//      both — and any unknown/future link type — MUST be excluded. A
	//      predicate shaped `NOT EXISTS (live link)` alone would silently
	//      eat every standalone file in the installation.
	//   2. No link has an empty entity_id — such a link is malformed data,
	//      not garbage. Its file is EXCLUDED here, which means KEPT and
	//      never surfaced to the worker: no probe can prove an empty id
	//      absent. (The worker re-asserts the same rule as
	//      defence-in-depth.)
	//   3. The file's created_at AND updated_at, and every link's
	//      created_at, are older than olderThan. A link created moments
	//      ago — a file attached concurrently with an entity delete — keeps
	//      the file for the whole age window.
	//   4. For every link, no row with id = entity_id exists in the table
	//      named by entity_type.
	//
	// SERVICE-MODE ONLY. The existence check must see every tenant and
	// group: links created before the attach endpoint checked the target's
	// group can point at an entity in ANOTHER group. A group-scoped (RLS)
	// implementation would read that live entity as nonexistent and hand
	// the GC a live file to delete.
	//
	// Implementations may return fewer rows than limit but never more,
	// and must order deterministically by the (created_at, id) KEYSET,
	// ascending, resuming strictly AFTER the `after` cursor when it is
	// non-zero.
	//
	// The cursor is not an optimization, it is a liveness requirement.
	// Most rows the GC re-verifies are KEPT, not deleted (a blocked
	// tenant, a suspended tenant, a pending_deletion group, an
	// unresolvable owner — several of which never clear), and a kept row
	// stays in the result set forever. Without a cursor, a bounded
	// oldest-first window is squatted on by those rows and no other
	// orphan in the installation is ever enumerated again — and in the
	// shipping report mode, where NOTHING is ever deleted, the very same
	// window would be re-reported on every tick.
	ListOrphanCandidates(ctx context.Context, olderThan time.Time, after OrphanCandidateCursor, limit int) ([]*models.FileEntity, error)

	// ExistingIDs returns the subset of ids that still have a file row,
	// in ONE `WHERE id = ANY($1)` round-trip (#2237). Service-mode only.
	//
	// It backs the orphan-file GC's thumbnail sweep, which needs to know
	// which of the file ids embedded in the keys it just listed are still
	// live. It is deliberately id-driven rather than tenant-driven: a
	// tenant-wide id dump would cost (and hold in memory) one entry per
	// file row in the installation's largest tenant to answer a question
	// about at most a few thousand listed keys.
	//
	// Missing / already-deleted ids are simply absent from the result; an
	// empty input returns (nil, nil) without a query.
	ExistingIDs(ctx context.Context, ids []string) ([]string, error)

	// ListIDsByOriginalPath returns the IDs of every file row — across
	// EVERY tenant and group — that carries the given files.original_path
	// (#2241). Service-mode only. An empty path returns (nil, nil) without
	// a query: "" is the sentinel for a row with no blob, not a key.
	//
	// It exists because a blob key is NOT row-unique. Uploads minted before
	// #2241 are keyed `t/<tenant>/files/<sanitized-name>-<unix SECONDS><ext>`
	// — no group segment, no row segment, no randomness — so two uploads of
	// the same filename inside one tenant in the same second (two members of
	// different groups; one user multi-selecting two same-named files) produce
	// two DISTINCT rows sharing one key. Every blob delete goes by key, and
	// `files` has no soft-delete, so deleting one of those rows would destroy
	// the other's bytes irreversibly. New uploads carry a UUID and cannot
	// collide, but the rows already in deployed databases can, forever.
	//
	// IDs rather than a count on purpose: the callers that need this are
	// deleting a BATCH (a group purge, a tenant hard-delete) and must ask
	// "does any row OUTSIDE my batch still reference this blob?". A count
	// cannot answer that without arithmetic over the batch, and getting that
	// arithmetic subtly wrong destroys user data.
	ListIDsByOriginalPath(ctx context.Context, originalPath string) ([]string, error)
}

// OrphanCandidateCursor is the keyset position a paged
// FileRegistry.ListOrphanCandidates scan resumes from: the last
// (created_at, id) the caller has already seen. The zero value means
// "start at the oldest row".
type OrphanCandidateCursor struct {
	CreatedAt time.Time
	ID        string
}

// IsZero reports whether the cursor names no position, i.e. the scan
// starts from the oldest candidate.
func (c OrphanCandidateCursor) IsZero() bool {
	return c.ID == "" && c.CreatedAt.IsZero()
}

// StorageBreakdown is the per-bucket byte count returned by
// FileRegistry.SumSizeBreakdown. Images / Documents / Other mirror
// models.FileCategory; Exports is files carrying an export link (export
// bundles, removed from Other to keep the user-meaningful tile semantics
// intact). Every stored file counts once, however many entities it is
// attached to.
//
// #1622 dropped the `invoices` bucket — legacy invoice rows are
// reclassified into Documents and carry the `invoice` tag.
//
// Trash holds the bytes of trashed files, whatever their category. They still
// occupy storage and count toward the total until they are purged, and the
// separate bucket tells a user what emptying Trash frees.
type StorageBreakdown struct {
	Images    int64 `json:"images"`
	Documents int64 `json:"documents"`
	Other     int64 `json:"other"`
	Exports   int64 `json:"exports"`
	Trash     int64 `json:"trash"`
}

// Total returns the sum of every bucket. Convenience for callers that
// want the headline number alongside the breakdown.
func (b StorageBreakdown) Total() int64 {
	return b.Images + b.Documents + b.Other + b.Exports + b.Trash
}

// TrashedFilesStats counts the files that went to Trash with one item.
type TrashedFilesStats struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
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
	// Kind filters to tags of the given kind (item-tags vs file-tags).
	// models.TagKindAny (the zero value) returns every tag regardless of
	// kind; the public list/autocomplete handlers require a concrete kind.
	Kind models.TagKind
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
// CommodityTagsTotal / FilesTagsTotal are the per-kind tag counts so each
// scoped view can show its own total.
type TagStats struct {
	TagsTotal          int `json:"tags_total"`
	CommodityTagsTotal int `json:"commodity_tags_total"`
	FileTagsTotal      int `json:"file_tags_total"`
	ItemsTagged        int `json:"items_tagged"`
	ItemsUntagged      int `json:"items_untagged"`
	FilesTagged        int `json:"files_tagged"`
	FilesUntagged      int `json:"files_untagged"`
}

// TagRegistry is the group-scoped catalogue of tags. The tag-string
// associations themselves continue to live in JSONB on commodities/files —
// only the metadata (label, color, usage) lives here.
type TagRegistry interface {
	Registry[models.Tag]

	// GetBySlug returns a tag by its (kind, slug) within the current group.
	// Returns ErrNotFound if no tag with that kind+slug exists. The same
	// slug can exist under two kinds, so kind is required to disambiguate.
	GetBySlug(ctx context.Context, kind models.TagKind, slug string) (*models.Tag, error)

	// ListPaginated returns paginated tags with optional q-search and
	// sorting. opts.Kind filters by kind (zero value = all kinds).
	ListPaginated(ctx context.Context, offset, limit int, opts TagListOptions) ([]*models.Tag, int, error)

	// Search returns tags of the given kind whose label or slug matches q
	// (case-insensitive substring), capped at limit. Used by the
	// autocomplete endpoint and ranked by usage desc + recency. Empty q
	// returns the most-used tags of that kind, also capped at limit.
	Search(ctx context.Context, q string, limit int, kind models.TagKind) ([]*models.Tag, error)

	// SlugsByLabel returns, sorted, the slugs of every tag of the given
	// kind in the current group whose label contains text as a
	// case-insensitive literal substring (% and _ match themselves).
	// Empty text returns nil. The item search uses it to find an item by
	// the label of one of its tags; kind keeps the item and file tag
	// namespaces apart.
	SlugsByLabel(ctx context.Context, kind models.TagKind, text string) ([]string, error)

	// GetUsage returns the reference count for a tag of the given kind
	// within the current group (commodities for commodity tags, files for
	// file tags). The irrelevant side of TagUsage is zero.
	GetUsage(ctx context.Context, kind models.TagKind, slug string) (TagUsage, error)

	// GetUsageBatch returns per-slug usage for the given slugs of the given
	// kind in a single round-trip. Used by the GET /tags?include=usage list
	// endpoint to avoid N+1 queries. Returned map is keyed by slug; missing
	// slugs map to a zero TagUsage so callers can read it unconditionally.
	GetUsageBatch(ctx context.Context, kind models.TagKind, slugs []string) (map[string]TagUsage, error)

	// GetStats returns the group-wide adoption summary for the Tags page
	// stats bar: total tags (overall + per kind), plus tagged/untagged
	// counts on commodities and files. "Tagged" = jsonb_array_length(tags) > 0.
	GetStats(ctx context.Context) (TagStats, error)

	// RewriteSlugReferences atomically rewrites the JSONB array entries from
	// oldSlug to newSlug for the current group on the table matching kind
	// (commodities for commodity tags, files for file tags), in the same
	// logical operation as the slug change on the tags row itself. Returns
	// (commodityRows, fileRows) touched — only the kind's side is non-zero.
	RewriteSlugReferences(ctx context.Context, kind models.TagKind, oldSlug, newSlug string) (int, int, error)

	// StripSlugReferences atomically removes every occurrence of slug from
	// the JSONB array on the table matching kind for the current group.
	// Used by force-delete. Returns (commodityRows, fileRows) — only the
	// kind's side is non-zero.
	StripSlugReferences(ctx context.Context, kind models.TagKind, slug string) (int, int, error)

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
	//
	// Saved views name item tags by slug. When the slug of an item tag
	// changes and rewriteViews is not nil, rewriteViews is called for every
	// saved view of the tag's group, the personal views of every member
	// included, and the views it changes are stored in the same transaction
	// under the group's saved view lock. The rename holds the locks of its
	// old and new slug until it commits, so another tag cannot take the
	// freed slug while the views still name it: a view keeps following the
	// tag it was saved with.
	//
	// It returns the tag as written and the slug it held when the lock was
	// taken, which is the slug the references were rewritten from.
	RenameAtomic(
		ctx context.Context,
		id, newLabel, newSlug string,
		newColor models.TagColor,
		rewriteViews SavedViewTagRewrite,
	) (renamed *models.Tag, previousSlug string, err error)

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
// CommodityLoanRegistry stores loans. The loans of a trashed item stay in the
// table and come back with the item; the group-wide reads (List, Count,
// ListPaginated, ListPendingReminders) leave them out. The per-item reads
// (ListByCommodity, GetOpenForCommodity, CountOpenByCommodity) answer for the
// ids they are given.
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

	// ListPendingReminders returns OPEN loans (`returned_at IS NULL`) whose
	// due_back_at is set AND falls into the requested LoanReminderKind
	// window AND whose matching idempotency flag is still false. Used by
	// the loan reminder worker (#1509) in service-mode across every group.
	// `now` is the worker's pinned clock; `dueSoonDays` is the window for
	// LoanReminderKindDueSoon (ignored for the overdue kind).
	//
	// Semantics:
	//   - LoanReminderKindOverdue: due_back_at < today (UTC) AND
	//     reminder_sent_overdue = false.
	//   - LoanReminderKindDueSoon: today <= due_back_at <= today + N days
	//     AND reminder_sent_due_soon = false.
	//
	// Loans with due_back_at IS NULL are excluded from both kinds
	// (open-ended). The "due-soon" window deliberately includes today
	// (a loan due today gets one reminder); once tomorrow rolls around
	// and the row is still open, it transitions to the overdue kind and
	// gets a second reminder against the separate flag.
	ListPendingReminders(ctx context.Context, kind LoanReminderKind, now time.Time, dueSoonDays int) ([]*models.CommodityLoan, error)

	// MarkReminderSent flips the matching reminder_sent_* boolean from
	// false to true atomically. The UPDATE filter includes the current
	// flag value (`reminder_sent_X = false`) so a concurrent worker
	// running the same sweep wins the row at most once. Returns
	// (true, nil) on a successful flip; (false, nil) when the row was
	// already flipped (another worker beat us) OR the loan disappeared.
	// Used by the worker right after a successful email enqueue.
	MarkReminderSent(ctx context.Context, loanID string, kind LoanReminderKind) (bool, error)
}

// LoanReminderKind narrows the kind of reminder the worker emits.
// Carried both on the registry method signatures (so the SQL filter +
// the column flip key off the same enum) and on the public worker
// stats so Prometheus metrics partition by kind.
type LoanReminderKind string

const (
	// LoanReminderKindOverdue selects loans whose due_back_at is in the past.
	LoanReminderKindOverdue LoanReminderKind = "overdue"
	// LoanReminderKindDueSoon selects loans whose due_back_at is between
	// today and today + N days (inclusive).
	LoanReminderKindDueSoon LoanReminderKind = "due_soon"
)

// IsValid reports whether the kind is one of the known values.
func (k LoanReminderKind) IsValid() bool {
	switch k {
	case LoanReminderKindOverdue, LoanReminderKindDueSoon:
		return true
	}
	return false
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
// CommodityServiceRegistry stores service records. The records of a trashed
// item stay in the table and come back with the item; the group-wide reads
// (List, Count, ListPaginated) leave them out.
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

// SupplyLinkRegistry is the group-scoped registry of commodity_supply_links
// (#1369). Supply links are tiny — label + URL + optional notes — and
// shaped like the loan registry: simple CRUD plus a per-commodity reader
// and a per-commodity bulk reorder.
type SupplyLinkRegistry interface {
	Registry[models.SupplyLink]

	// ListByCommodity returns all supply links for one commodity, ordered
	// by sort_order ASC, created_at ASC. Drives the per-item Supplies card
	// and form section.
	ListByCommodity(ctx context.Context, commodityID string) ([]*models.SupplyLink, error)

	// ReorderForCommodity replaces sort_order for every supply link of
	// the given commodity with its position in `orderedIDs`. Densely
	// renumbered 0..N-1. Any id in the input that does not belong to the
	// commodity is rejected with ErrNotFound — no partial reorder. Any
	// id of this commodity NOT present in the input keeps its prior
	// sort_order so a partial reorder cannot drop rows; callers should
	// pass the full current set when they want a true permutation.
	ReorderForCommodity(ctx context.Context, commodityID string, orderedIDs []string) error

	// CountByCommodity returns, for the given commodity ids, the per-id
	// count of supply links. Drives the list-page "N supplies" badge in
	// a single round-trip (parallels CommodityLoanRegistry.CountOpenByCommodity).
	// Empty input returns an empty map; missing ids map to 0.
	CountByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error)
}

// SavedViewTally counts the views of one group. CreateChecked and UpdateFunc
// take it under the group's write lock, so a limit checked against it holds
// when the write lands.
type SavedViewTally struct {
	// Own counts the views the registry's user created in the group.
	Own int
	// Shared counts the views shared with the group.
	Shared int
}

// SavedViewPlacementPatch changes where a member shows a view. A nil field
// keeps the stored value, or false when the member has no placement yet.
type SavedViewPlacementPatch struct {
	ShowInSidebar   *bool
	ShowOnDashboard *bool
}

// SavedViewRegistry stores the saved views of one group and where each
// member shows them. A user registry sees the views its user created and
// every view shared with the group; Get, Update and Delete answer
// ErrNotFound for another member's personal view, so a view a member cannot
// see is indistinguishable from one that does not exist. Who may change a
// view they can see is decided by the service. A service registry sees every
// row of every group.
type SavedViewRegistry interface {
	// Create stores a view after every view of the group: its SortOrder is
	// one past the highest in the group, whatever the caller set. Update
	// keeps the stored SortOrder; Reorder is the only way to change it.
	// Get, List, Update, Delete and Count follow the visibility rule above.
	// Deleting a view deletes its placements.
	Registry[models.SavedView]

	// CreateChecked is Create under the group's write lock, for a user
	// registry. check gets the tally taken under that lock; an error from
	// it is returned and nothing is stored. When placement is not nil it is
	// stored as the user's placement of the new view in the same
	// transaction.
	CreateChecked(ctx context.Context, view models.SavedView, placement *SavedViewPlacementPatch, check func(SavedViewTally) error) (*models.SavedView, error)

	// UpdateFunc reads the visible view id under the group's write lock and
	// writes back what mutate leaves of it. mutate also gets the tally taken
	// under the lock; an error from it is returned and nothing is written.
	// The id, tenant, group, creator, SortOrder and CreatedAt are kept
	// whatever mutate does. When the view stops being shared, the
	// placements other members had for it are deleted. When placement is
	// not nil and the user still sees the view as written, it is stored as
	// the user's placement in the same transaction, so the view and the
	// placement are written together or not at all.
	UpdateFunc(
		ctx context.Context,
		id string,
		placement *SavedViewPlacementPatch,
		mutate func(view *models.SavedView, tally SavedViewTally) error,
	) (*models.SavedView, error)

	// Placements returns the placements of the user registry's user for the
	// views they see, keyed by view ID. A view without an entry is shown
	// nowhere.
	Placements(ctx context.Context) (map[string]models.SavedViewPlacement, error)

	// SetPlacement changes where the user registry's user shows a view they
	// see and returns the stored placement. An unknown or invisible view
	// fails with ErrNotFound.
	SetPlacement(ctx context.Context, viewID string, patch SavedViewPlacementPatch) (*models.SavedViewPlacement, error)

	// Reorder gives the views named in ids, which must be visible and
	// distinct, the positions they hold now, sorted and handed out in the
	// order of ids. Views not named keep their positions, so a member can
	// reorder the views they may edit without moving the others. An id that
	// is unknown, invisible or repeated fails with ErrNotFound and nothing
	// is changed.
	Reorder(ctx context.Context, ids []string) error
}

// SavedViewTagRewrite returns the query a saved view gets when the item tag
// slug from is renamed to to, and whether the view changed. See
// TagRegistry.RenameAtomic.
type SavedViewTagRewrite func(view models.SavedView, from, to string) (query string, changed bool)

// MaintenanceListOptions narrows the result of
// MaintenanceScheduleRegistry.ListPaginated.
type MaintenanceListOptions struct {
	// DueBefore filters out schedules whose NextDueAt is strictly
	// after the given date. Empty = no filter. Used by the group-wide
	// "upcoming maintenance" list. Lexicographic comparison is correct
	// for ISO YYYY-MM-DD dates.
	DueBefore string
	// EnabledOnly restricts the result to enabled schedules.
	EnabledOnly bool
}

// MaintenanceScheduleRegistry is the group-scoped registry of
// maintenance_schedules. Schedules are simple row-based entities — the
// service layer enforces the small set of invariants (positive interval,
// next_due_at recompute on MarkDone) rather than the DB.
// MaintenanceScheduleRegistry stores maintenance schedules. The schedules of a
// trashed item stay in the table and come back with the item; the group-wide
// reads (List, Count, ListPaginated) leave them out, which is what keeps the
// maintenance reminder worker quiet about them.
type MaintenanceScheduleRegistry interface {
	Registry[models.MaintenanceSchedule]

	// ListByCommodity returns all schedules for a single commodity,
	// ordered by next_due_at ascending. Used by the per-item
	// Maintenance section.
	ListByCommodity(ctx context.Context, commodityID string) ([]*models.MaintenanceSchedule, error)

	// ListPaginated returns a paginated, group-wide list of schedules
	// ordered by next_due_at ascending. Pass a zero
	// MaintenanceListOptions for "all rows".
	ListPaginated(ctx context.Context, offset, limit int, opts MaintenanceListOptions) ([]*models.MaintenanceSchedule, int, error)

	// CountByCommodity returns, for the given commodity ids, the
	// per-id count of schedules. Used by the list page to drive the
	// "has maintenance" badge in a single round-trip rather than N+1.
	// Empty input returns an empty map; missing ids map to 0.
	CountByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error)
}

// MaintenanceReminderRegistry is the worker-only registry that records
// "reminder X for schedule Y at threshold Z has been emitted" rows.
// The (schedule_id, threshold_days) tuple is unique — Create returns
// (false, nil) for the loser of a race so the worker can treat the
// happy path and the race-loser path identically (both mean "no email
// is needed from this tick").
//
// Reset semantics: when the user marks a schedule done the service
// calls DeleteBySchedule so the next cycle starts with a clean slate.
// On commodity / schedule hard-delete the rows cascade away via the
// FK.
//
// All operations run under the background-worker RLS bypass. There is
// no user-facing surface on this table.
type MaintenanceReminderRegistry interface {
	// HasSent reports whether a reminder row already exists for the
	// given (schedule, threshold) tuple.
	HasSent(ctx context.Context, scheduleID string, thresholdDays int) (bool, error)

	// HasSentInRange reports whether a reminder row that is not silent
	// exists for the schedule with fromDays <= threshold < belowDays. The
	// worker uses it to skip a lead time once an email for a closer one
	// has gone out (#2805). Silent rows do not count: no email went out
	// for them.
	HasSentInRange(ctx context.Context, scheduleID string, fromDays, belowDays int) (bool, error)

	// CreateOnce attempts to insert the reminder row. Returns
	// (true, nil) if this call won the insert and the caller may
	// proceed to send the email. Returns (false, nil) when a row for
	// the same tuple already exists (idempotency).
	CreateOnce(ctx context.Context, reminder models.MaintenanceReminder) (bool, error)

	// DeleteBySchedule removes every reminder row for the given
	// schedule. Called by the service when the user marks the
	// schedule done so the next cycle gets a clean idempotency state.
	// Returns the number of rows deleted.
	DeleteBySchedule(ctx context.Context, scheduleID string) (int, error)
}

// CommodityReminderRegistry is the group-scoped registry of
// commodity_reminders, the dated reminders a user sets on an item. In
// service mode List returns every reminder across tenants; the reminder
// worker reads it that way.
type CommodityReminderRegistry interface {
	Registry[models.CommodityReminder]

	// ListByCommodity returns the reminders of one item, ordered by
	// due_on ascending.
	ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityReminder, error)
}

// CommodityDraftRegistry is the group-scoped registry of commodity_drafts,
// the unfinished items saved on the server (#2812). A draft is not an item:
// no CommodityRegistry read returns it.
//
// List and Count see open and promoted drafts; ListOpen sees open drafts
// only. A promoted draft keeps its row with the values cleared, so a
// repeated create with its client key or a repeated promotion finds it.
type CommodityDraftRegistry interface {
	Registry[models.CommodityDraft]

	// CreateOrGet stores a new open draft unless the group already has a
	// draft with the same ClientKey, which is returned unchanged with
	// created=false. The registry sets the ids, tenant, group, author,
	// status, revision and timestamps; a backup restore passes the archived
	// revision and timestamps through WithRestoredDraft.
	CreateOrGet(ctx context.Context, draft models.CommodityDraft) (stored *models.CommodityDraft, created bool, err error)

	// GetByClientKey returns the group's draft with the key; ErrNotFound
	// when there is none.
	GetByClientKey(ctx context.Context, clientKey string) (*models.CommodityDraft, error)

	// ListOpen returns a page of the group's open drafts, most recently
	// saved first, with the total.
	ListOpen(ctx context.Context, offset, limit int) ([]*models.CommodityDraft, int, error)

	// OpenIDs returns the ids of every open draft of the group, in the order
	// of ListOpen. It reads ids only, for a caller that counts the drafts
	// or merges them with other records without loading their values.
	OpenIDs(ctx context.Context) ([]string, error)

	// SaveValues replaces the values of an open draft and increments its
	// revision. revision is the revision the values were based on:
	// ErrDraftRevisionConflict when it is not the current one,
	// ErrDraftPromoted when the draft is promoted, ErrDraftDiscarded when
	// its discard has begun, ErrNotFound when it does not exist.
	SaveValues(ctx context.Context, id string, revision int, values models.CommodityDraftValues) (*models.CommodityDraft, error)

	// Promote turns an open draft into an item in one step: it creates the
	// item from commodity the way CommodityRegistry.Create does, moves the
	// draft's file links to the item, and marks the draft promoted with its
	// values cleared and promotionKey stored. Either all of it happens or
	// none of it.
	//
	// A repeat with the same promotionKey changes nothing and returns the
	// promoted draft with a nil Commodity, whatever revision says; the
	// caller reads the item through PromotedCommodityID. Another key on a
	// promoted draft gets ErrDraftPromoted. On an open draft revision must
	// be the current one (ErrDraftRevisionConflict); a draft whose discard
	// has begun gets ErrDraftDiscarded. The caller validates the item and
	// normalizes its tags beforehand.
	Promote(ctx context.Context, id string, revision int, promotionKey string, commodity models.Commodity) (DraftPromotion, error)

	// AttachFile links a file to an open draft, holding the draft row so a
	// promotion or a discard cannot happen between the check and the
	// insert. It returns the existing link with created=false when the file
	// is already attached; ErrDraftPromoted or ErrDraftDiscarded when the
	// draft is not open, ErrNotFound when the draft or the file is not in
	// the group.
	AttachFile(ctx context.Context, draftID string, link models.FileLink) (stored *models.FileLink, created bool, err error)

	// MarkDiscarding begins the discard of an open draft: from then on it
	// cannot be saved, promoted or given files, and no list shows it. On a
	// draft already being discarded it returns the draft unchanged, so a
	// discard that stopped half way can be finished. ErrDraftPromoted when
	// the draft is an item, ErrNotFound when it does not exist.
	MarkDiscarding(ctx context.Context, id string) (*models.CommodityDraft, error)

	// DeleteDiscarding deletes a draft whose discard has begun; ErrNotFound
	// when there is no such draft.
	DeleteDiscarding(ctx context.Context, id string) error

	// ListDiscarding returns, oldest first, at most limit of the group's
	// drafts whose discard began before the given time and did not finish.
	// MarkDiscarding sets UpdatedAt, so it is the time the discard began.
	ListDiscarding(ctx context.Context, before time.Time, limit int) ([]*models.CommodityDraft, error)
}

// IntakeSourceInput is one source of a new intake session: the files the AI
// scan reads together.
type IntakeSourceInput struct {
	// FileIDs are files of the group, uploaded beforehand. Each is linked
	// to the source with entity type "intake"; images go to the "images"
	// bucket and everything else to "documents".
	FileIDs []string
	// OneItem says the files show one item.
	OneItem bool
	// ExternalSystem, ExternalInstance and ExternalDocumentID identify a
	// document another system delivered (#2809); empty for an upload. A
	// document id needs the system it belongs to.
	ExternalSystem     string
	ExternalInstance   string
	ExternalDocumentID string
	// Delivered, when set, holds the products another system delivered for
	// the source in place of a scan (#2809). The source is stored scanned
	// with these candidates, so the worker never takes it and it does not
	// count against the queue limit.
	Delivered *IntakeScan
}

// CheckIntakeSourceInputs checks what both backends require of the sources
// of a new session before anything is written: a document id comes with the
// system it belongs to.
func CheckIntakeSourceInputs(sources []IntakeSourceInput) error {
	for _, src := range sources {
		if src.ExternalDocumentID != "" && src.ExternalSystem == "" {
			return errxtrace.Classify(ErrFieldRequired, errx.Attrs("field_name", "ExternalSystem"))
		}
	}
	return nil
}

// QueuedIntakeSources counts the sources of a new session that go to the scan
// queue: all but the delivered ones.
func QueuedIntakeSources(sources []IntakeSourceInput) int {
	n := 0
	for _, src := range sources {
		if src.Delivered == nil {
			n++
		}
	}
	return n
}

// IntakeScan is what the scan of one source produced.
type IntakeScan struct {
	// Candidates are the detected products in order. The registry sets
	// their ids, scope, session, source, position and outcome; Fields,
	// Selected, AcceptedFields and Quantity are stored as given.
	Candidates []models.IntakeCandidate
	Warnings   []models.IntakeWarning
}

// IntakeCandidateReview is a member's review of a detected product. A nil
// field is left unchanged.
type IntakeCandidateReview struct {
	Selected       *bool
	AcceptedFields *[]string
	Quantity       *int
}

// IntakeSweep is what one SweepStuckSources changed.
type IntakeSweep struct {
	// Changed counts the sources the sweep changed in any way.
	Changed int
	// Failed counts the sources among them that used their attempts and
	// failed for good (#1307).
	Failed int
}

// IntakeRegistry is the group-scoped registry of intake sessions with their
// sources and detected products (#2810). The embedded Registry works on the
// sessions; deleting a session deletes its sources, its candidates and the
// "intake" file links of its sources, and leaves the files and every draft
// made from it alone.
//
// A user registry acts in the group of its context. The service registry
// serves the background worker: the queue methods (SweepStuckSources,
// ClaimNextSource, StoreScan, RequeueSource, FailSource) work across groups
// and address sources by id.
type IntakeRegistry interface {
	Registry[models.IntakeSession]

	// CreateSession stores a session, its sources in the given order and
	// the links of their files, all or nothing. When the group already has a
	// session with the same ClientKey, that session is returned unchanged
	// with created=false, whatever the sources given. Every file must be a
	// live file of the group (ErrNotFound otherwise). A source whose
	// external identity the group already has fails with ErrAlreadyExists,
	// one with a document id but no system with ErrFieldRequired.
	//
	// With maxQueued above zero, a new session whose sources would bring the
	// group's queued sources (see CountQueued) above maxQueued fails with
	// ErrIntakeQueueFull. The count and the insert are atomic: two sessions
	// created at once cannot both pass on the same free room.
	CreateSession(ctx context.Context, session models.IntakeSession, sources []IntakeSourceInput, maxQueued int) (stored *models.IntakeSession, created bool, err error)

	// GetSessionByClientKey returns the group's session with the key;
	// ErrNotFound when there is none.
	GetSessionByClientKey(ctx context.Context, clientKey string) (*models.IntakeSession, error)

	// ListSessions returns a page of the group's sessions, newest first,
	// with the total.
	ListSessions(ctx context.Context, offset, limit int) ([]*models.IntakeSession, int, error)

	// ListOpenSessions returns the group's open sessions, newest first: a
	// source waits for its scan, is being scanned or failed, or a candidate
	// has no outcome yet or is kept with a failed draft. It is the set
	// models.IntakeSessionSummary.Open reports, read in one query for the
	// inbox count. It needs a group: the service registry gets
	// ErrFieldRequired rather than the sessions of every group.
	ListOpenSessions(ctx context.Context) ([]*models.IntakeSession, error)

	// ListSources returns the sources of the given sessions, ordered by
	// session and position.
	ListSources(ctx context.Context, sessionIDs ...string) ([]*models.IntakeSource, error)

	// GetSource returns one source; ErrNotFound when it is not in the
	// group.
	GetSource(ctx context.Context, id string) (*models.IntakeSource, error)

	// GetSourceByExternalID returns the group's source with the external
	// identity; ErrNotFound when there is none. It needs a group: the
	// service registry gets ErrFieldRequired, since the identity is unique
	// per group only.
	GetSourceByExternalID(ctx context.Context, system, instance, documentID string) (*models.IntakeSource, error)

	// ListCandidates returns the candidates of the given sessions, ordered
	// by source position and candidate position.
	ListCandidates(ctx context.Context, sessionIDs ...string) ([]*models.IntakeCandidate, error)

	// GetCandidate returns one candidate; ErrNotFound when it is not in
	// the group.
	GetCandidate(ctx context.Context, id string) (*models.IntakeCandidate, error)

	// ReviewCandidate applies a member's review. A skipped candidate that
	// is selected again goes back to pending. ErrIntakeCandidateFinished
	// when the candidate has its draft.
	ReviewCandidate(ctx context.Context, id string, review IntakeCandidateReview) (*models.IntakeCandidate, error)

	// SetCandidateOutcome records what became of a candidate. draftID is
	// stored with the created outcome and errorCode with the failed one.
	SetCandidateOutcome(ctx context.Context, id string, outcome models.IntakeCandidateOutcome, draftID *string, errorCode string) (*models.IntakeCandidate, error)

	// CancelSession stops the session's queued sources: a pending source
	// is canceled at once; a source in a scan is canceled too, and the
	// result of that scan is still stored when it arrives, so a paid scan
	// is not lost. It returns how many sources it canceled.
	CancelSession(ctx context.Context, sessionID string) (int, error)

	// ResumeSession undoes a cancel and retries failures. A source canceled
	// while its scan runs goes back to scanning under the same claim, so the
	// running scan stays the only one. Other canceled sources and failed
	// sources go back in the queue, due at now, with a fresh allowance of
	// attempts (RetryBase set to Attempts). Scanned sources are left alone,
	// so a resume never scans a file twice. It returns how many sources it
	// changed.
	ResumeSession(ctx context.Context, sessionID string, now time.Time) (int, error)

	// RescanSource puts a scanned, failed or canceled source back in the
	// queue with a fresh allowance of attempts and deletes its candidates,
	// so the next scan replaces them. ErrIntakeSourceBusy when the source is
	// queued or a canceled scan of it still runs; ErrIntakeSourceHasDrafts
	// when one of its candidates has its draft, or a draft with a
	// candidate's client key exists in the group and is not being discarded
	// (a draft made by a call that stopped before it recorded the outcome).
	RescanSource(ctx context.Context, id string, now time.Time) (*models.IntakeSource, error)

	// CountQueued counts the group's sources that wait for or are in a
	// scan.
	CountQueued(ctx context.Context) (int, error)

	// SweepStuckSources handles the claims taken before claimedBefore that
	// no worker finished. A scanning source goes back in the queue, due at
	// now, to wait for a retry, unless it used maxAttempts counted attempts
	// (models.IntakeSource.CountedAttempts): then it fails with errorCode, so
	// a file that kills its worker is not scanned forever. A source canceled
	// during such a scan loses the claim, so a resume can queue it. It
	// returns how many sources it changed and how many of them failed.
	SweepStuckSources(ctx context.Context, claimedBefore, now time.Time, maxAttempts int, errorCode string) (IntakeSweep, error)

	// ClaimNextSource takes the pending source that has been due the
	// longest, marks it scanning and counts an attempt. Two workers never
	// get the same source. ErrNotFound when nothing is due.
	ClaimNextSource(ctx context.Context, now time.Time) (*models.IntakeSource, error)

	// StoreScan stores the outcome of a scan and marks the source scanned.
	// attempt is the source's Attempts as ClaimNextSource returned it; when
	// the source is no longer that scan (neither scanning nor canceled with
	// that attempt), nothing is stored and ErrIntakeSourceChanged is
	// returned.
	StoreScan(ctx context.Context, sourceID string, attempt int, scan IntakeScan, now time.Time) error

	// RequeueSource puts a scanning source back in the queue, due at next,
	// with the reason it waits (models.IntakeWait*). A wait for the rate
	// limit does not count the attempt against the limit (RetryBase goes up
	// by one), as it is not the source's fault. A source that has moved on
	// (see StoreScan) is left alone and ErrIntakeSourceChanged returned; so
	// is one canceled during the scan, which only loses the claim.
	RequeueSource(ctx context.Context, sourceID string, attempt int, next time.Time, waitReason string) error

	// FailSource marks a scanning source failed with the error code. A
	// source that has moved on is left alone and ErrIntakeSourceChanged
	// returned; so is one canceled during the scan, which only loses the
	// claim.
	FailSource(ctx context.Context, sourceID string, attempt int, errorCode string) error
}

// IntakeDocumentProgress is what a delivery records on its document as it
// goes. A nil field is left as it is.
type IntakeDocumentProgress struct {
	FileID    *string
	SessionID *string
	DraftID   *string
	// CommodityID is the item the draft became, for a document that missed
	// the promotion.
	CommodityID *string
}

// IntakeDocumentRegistry is the group-scoped registry of the documents other
// systems delivered (#2809). The embedded Registry serves the backup, the
// restore and the purges. Every other method needs a group: the service
// registry gets ErrFieldRequired, since an external identity is unique per
// group only.
//
// CommodityDraftRegistry.Promote sets CommodityID on the document whose
// draft it promotes, in its own transaction.
type IntakeDocumentRegistry interface {
	Registry[models.IntakeDocument]

	// CreateOrGet stores a new document unless the group already has one
	// with the same external identity, which is returned unchanged with
	// created=false. The registry sets the ids, tenant, group and author;
	// the timestamps are now unless the caller set them, as a restore does.
	CreateOrGet(ctx context.Context, doc models.IntakeDocument) (stored *models.IntakeDocument, created bool, err error)

	// GetByExternalID returns the group's document with the identity;
	// ErrNotFound when there is none.
	GetByExternalID(ctx context.Context, system, instance, documentID string) (*models.IntakeDocument, error)

	// RecordProgress fills in the fields of progress that are still empty
	// and returns the document. A field that is already set keeps its value,
	// so of two deliveries racing on one document the first to record a
	// file, a session or a draft wins, and the other finds it in the result.
	// Recording a draft that is already promoted records its item too, so a
	// promotion that ran before the draft was recorded is not lost; the
	// draft is locked as a promotion locks it, so one of the two always
	// sees the other.
	RecordProgress(ctx context.Context, id string, progress IntakeDocumentProgress) (*models.IntakeDocument, error)

	// ListByDrafts returns the group's documents whose DraftID is one of
	// draftIDs, oldest first.
	ListByDrafts(ctx context.Context, draftIDs ...string) ([]*models.IntakeDocument, error)

	// ListByCommodities returns the group's documents whose CommodityID is
	// one of commodityIDs, oldest first.
	ListByCommodities(ctx context.Context, commodityIDs ...string) ([]*models.IntakeDocument, error)
}

// DraftPromotion is the result of CommodityDraftRegistry.Promote.
type DraftPromotion struct {
	// Draft is the draft after the call, in the promoted state.
	Draft *models.CommodityDraft
	// Commodity is the item this call created. It is nil when the draft
	// was promoted before the call.
	Commodity *models.Commodity
}

// SellerRegistry is the group-scoped registry of sellers (#2804). Create and
// Update normalize the seller (models.Seller.Normalize) and fail with
// ErrAlreadyExists when another seller of the group has the same name key.
// Delete leaves the seller's items without a seller.
type SellerRegistry interface {
	Registry[models.Seller]

	// GetByName returns the seller whose name key equals the key of name,
	// or ErrNotFound.
	GetByName(ctx context.Context, name string) (*models.Seller, error)

	// ItemCounts returns, per seller ID, how many items of the group point
	// at that seller, drafts and inactive items included and items in Trash
	// left out. A seller without items is absent from the map.
	ItemCounts(ctx context.Context) (map[string]int, error)
}

// ItemTypeUsage counts the items and the open drafts of a group that have
// one item type.
type ItemTypeUsage struct {
	// Live counts the items outside Trash, drafts and inactive items
	// included.
	Live int
	// Trashed counts the items in Trash.
	Trashed int
	// Drafts counts the open drafts (CommodityDraftRegistry) whose values
	// name the type.
	Drafts int
}

// Total is the number of items, in Trash or not, that have the type.
func (u ItemTypeUsage) Total() int {
	return u.Live + u.Trashed
}

// InUse reports whether an item, in Trash or not, or an open draft has the
// type.
func (u ItemTypeUsage) InUse() bool {
	return u.Total() > 0 || u.Drafts > 0
}

// ItemTypeRegistry is the group-scoped registry of item types (#2802).
//
// Create and Update normalize the type (models.ItemType.Normalize) and fail
// with ErrAlreadyExists when another type of the group has the same name
// key. Create gives the type a slug made from its name, unique in the
// group and never one of a built-in type, unless the caller sets one (the
// backup restore keeps the archived slug when it is free). Update keeps the
// stored slug. Create fails with ErrItemTypeLimit when the group already has
// models.MaxItemTypesPerGroup types.
//
// A group with no item types has never been seeded: the user registry's
// List, Create and the commodity registries' writes of a built-in slug seed
// the built-in types into it first. Delete is DeleteReassigning without a
// target.
type ItemTypeRegistry interface {
	Registry[models.ItemType]

	// GetBySlug returns the type with the given slug, or ErrNotFound.
	GetBySlug(ctx context.Context, slug models.CommodityType) (*models.ItemType, error)

	// GetByName returns the type whose name key equals the key of name, or
	// ErrNotFound. A built-in type with no name of its own is found by its
	// English default name.
	GetByName(ctx context.Context, name string) (*models.ItemType, error)

	// ItemCounts returns, per type slug, how many items and open drafts of
	// the group have that type. A slug that neither has is absent from the
	// map.
	ItemCounts(ctx context.Context) (map[models.CommodityType]ItemTypeUsage, error)

	// DeleteReassigning deletes the type with id. When items of the group,
	// those in Trash included, or its open drafts have it, they are first
	// given the type reassignTo, in the same transaction, and the count of
	// moved items is returned; each moved draft gets a new revision, so a
	// save from a tab that loaded it before is refused. With an empty
	// reassignTo the delete fails with ErrItemTypeInUse instead. A
	// reassignTo that is not another type of the group fails with
	// ErrInvalidInput. The last type of a group cannot be deleted
	// (ErrLastItemType).
	DeleteReassigning(ctx context.Context, id, reassignTo string) (int, error)

	// EnsureBuiltins seeds the built-in types into the registry's group when
	// the group has no item types, and returns how many it created.
	EnsureBuiltins(ctx context.Context) (int, error)

	// BackfillBuiltins seeds the built-in types into every group that has
	// no item types, one group per transaction, and returns how many types
	// it created. Service registry only.
	BackfillBuiltins(ctx context.Context) (int, error)
}

// CustomFieldUsage counts the items and the open drafts of a group that
// have a value for one custom field.
type CustomFieldUsage struct {
	// Live counts the items outside Trash, drafts and inactive items
	// included.
	Live int
	// Trashed counts the items in Trash.
	Trashed int
	// Drafts counts the open drafts (CommodityDraftRegistry) that hold a
	// value of the field (models.DraftCustomFieldHasValue).
	Drafts int
	// Options counts, for a select field, the items with each option ID,
	// those in Trash included. Nil for other types.
	Options map[string]int
}

// Total is the number of items, in Trash or not, that have a value.
func (u CustomFieldUsage) Total() int {
	return u.Live + u.Trashed
}

// InUse reports whether an item, in Trash or not, or an open draft has a
// value of the field.
func (u CustomFieldUsage) InUse() bool {
	return u.Total() > 0 || u.Drafts > 0
}

// CustomFieldRegistry is the group-scoped registry of custom field
// definitions and of the item type templates that name them (#2803). The
// values themselves belong to the items: CommodityRegistry reads and writes
// them as models.Commodity.CustomFields.
//
// Create and Update normalize the definition
// (models.CustomFieldDefinition.Normalize), validate it and fail with
// ErrAlreadyExists when another field of the group has the same name key.
// A template entry naming no item type of the group fails with
// ErrInvalidInput. Update keeps the stored type and creation time; the
// values of options it drops are deleted in the same transaction, and so are
// template defaults that name them. Get and List return the definitions
// with their templates; List orders them by name key, then ID.
//
// Delete deletes a field that no item and no open draft has a value of; a
// field that items, those in Trash included, or open drafts have a value of
// fails with ErrCustomFieldInUse and nothing is deleted.
type CustomFieldRegistry interface {
	Registry[models.CustomFieldDefinition]

	// GetByName returns the field whose name key equals the key of name,
	// or ErrNotFound.
	GetByName(ctx context.Context, name string) (*models.CustomFieldDefinition, error)

	// Usage returns, per field ID, how many items and open drafts of the
	// group have a value for the field. A field without values is absent
	// from the map.
	Usage(ctx context.Context) (map[string]CustomFieldUsage, error)

	// DeleteWithValues deletes the field with id together with its values
	// and template entries, in one transaction, and returns how many items
	// and open drafts had a value. It also takes the field's entry out of
	// every open draft that has one, value or not, and gives each such
	// draft a new revision. expected is how many items and drafts together
	// the caller confirmed to lose a value, or AnyValueCount; when another
	// number has a value, it fails with ErrCustomFieldInUse and nothing is
	// deleted.
	DeleteWithValues(ctx context.Context, id string, expected int) (int, error)

	// AddTemplates puts the field with id on the template of each item
	// type in itemTypeIDs it is not on yet, with no default, and returns
	// how many entries it added. The entries the field has keep their
	// defaults. An ID that names no item type of the group fails with
	// ErrInvalidInput and adds nothing.
	AddTemplates(ctx context.Context, id string, itemTypeIDs []string) (int, error)
}

// AnyValueCount lets CustomFieldRegistry.DeleteWithValues delete the values
// of any number of items.
const AnyValueCount = -1

// CommodityReminderSendRegistry records which reminder emails the worker
// has claimed. One row per (reminder, occurrence, lead time); the unique
// index on that tuple keeps an email from going out twice, across
// restarts and across worker replicas. Rows go away with their reminder.
//
// All operations run under the background-worker RLS bypass. There is no
// user-facing surface on this table.
type CommodityReminderSendRegistry interface {
	// HasSentAtOrBelow reports whether an email for the occurrence has
	// already been claimed for a lead time of leadDays or less. Such an
	// email is at least as recent as the one leadDays would send, so the
	// worker skips the larger lead time.
	HasSentAtOrBelow(ctx context.Context, reminderID string, occurrenceOn models.Date, leadDays int) (bool, error)

	// CreateOnce claims the email for the send's (reminder, occurrence,
	// lead time). It returns (true, nil) when this call inserted the row
	// and the caller may send, and (false, nil) when the row already
	// exists.
	CreateOnce(ctx context.Context, send models.CommodityReminderSend) (bool, error)

	// Release deletes a claim, so that the next sweep retries an email
	// whose enqueue failed.
	Release(ctx context.Context, reminderID string, occurrenceOn models.Date, leadDays int) error

	// DeleteByReminder removes every row of a reminder and returns how many
	// it removed. The purgers call it before they delete the reminder.
	DeleteByReminder(ctx context.Context, reminderID string) (int, error)
}

type ThumbnailGenerationJobRegistry interface {
	Registry[models.ThumbnailGenerationJob]

	// GetPendingJobs returns pending thumbnail generation jobs ordered by priority and creation time
	GetPendingJobs(ctx context.Context, limit int) ([]*models.ThumbnailGenerationJob, error)

	// GetJobByFileID returns the thumbnail generation job for a specific file
	GetJobByFileID(ctx context.Context, fileID string) (*models.ThumbnailGenerationJob, error)

	// ListByFileID returns every thumbnail generation job referencing the
	// given file. A file can own more than one job (e.g. a failed job plus a
	// retry), so the file-cleanup path uses this to find every job whose
	// concurrency slots must be cleared before the jobs are deleted. Returns
	// an empty slice (no ErrNotFound) when no job references the file.
	ListByFileID(ctx context.Context, fileID string) ([]*models.ThumbnailGenerationJob, error)

	// UpdateJobStatus updates the status of a thumbnail generation job
	UpdateJobStatus(ctx context.Context, jobID string, status models.ThumbnailGenerationStatus, errorMessage string) error

	// CleanupCompletedJobs removes completed/failed jobs older than the specified duration
	CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) error

	// DeleteByFileID removes every thumbnail generation job referencing the
	// given file. Used by the file-deletion cleanup path to break the
	// thumbnail_generation_jobs.file_id -> files(id) FK (NO ACTION) before
	// the file row is removed. Idempotent: deleting when no job references
	// the file is a no-op and returns nil (no ErrNotFound on zero matches).
	DeleteByFileID(ctx context.Context, fileID string) error
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

	// DeleteByJobID removes every concurrency slot referencing the given
	// thumbnail generation job. Used by the thumbnail-cleanup path to break
	// the user_concurrency_slots.job_id -> thumbnail_generation_jobs(id) FK
	// (NO ACTION) before the job row is removed. Idempotent: deleting when
	// no slot references the job is a no-op and returns nil.
	DeleteByJobID(ctx context.Context, jobID string) error
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

	// HasActive reports whether any restore operation is pending or running.
	// Answering that with List means reading every operation and its steps to
	// learn one bit, and the cost grows with the history (#1314).
	HasActive(ctx context.Context) (bool, error)

	// ClaimPending moves an operation from pending to running and reports
	// whether this caller is the one that moved it. Two workers racing for
	// the same row get one true and one false, rather than both replaying a
	// restore over live data (#2472).
	ClaimPending(ctx context.Context, id string) (bool, error)
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

	// DeleteAuditRowsByGroup removes every currency-migration audit row
	// belonging to the given (tenant, group). Used by the group-deletion
	// cleanup path to break the
	// currency_migration_audit_rows.group_id -> location_groups(id) FK
	// (NO ACTION) before the group is removed. The audit rows do NOT
	// cascade from the group (they only cascade from the migration row via
	// currency_migration_audit_rows.migration_id), so they must be cleared
	// explicitly. Returns the number of rows deleted. Idempotent: zero
	// matches returns (0, nil).
	DeleteAuditRowsByGroup(ctx context.Context, tenantID, groupID string) (int, error)

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

	// DeleteFinishedBefore deletes at most limit completed and failed
	// migrations, discarded or not, that finished before the cutoff,
	// together with their audit rows, and returns how many it deleted.
	// A migration that has no completion time counts from its creation.
	// A pending or running migration is never deleted, and neither is
	// one a group still references through
	// location_groups.currency_migration_id. A limit of zero or less
	// deletes every match. Service registry only.
	DeleteFinishedBefore(ctx context.Context, before time.Time, limit int) (int, error)

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

	// ListByRestoreOperations returns the steps of several operations grouped
	// by operation id. Listing them one at a time while iterating the
	// operations nests a query inside an open cursor (#2469).
	ListByRestoreOperations(ctx context.Context, restoreOperationIDs []string) (map[string][]models.RestoreStep, error)

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

	// ListAdmin returns a paginated, filtered, and sorted listing of every
	// tenant in the deployment alongside per-tenant computed counts
	// (user_count, group_count). The endpoint behind this method
	// (/api/v1/admin/tenants — #1746) crosses tenants by design, so
	// implementations MUST return rows across all tenants regardless of
	// caller membership. The postgres TenantRegistry runs against
	// NonRLSRepository (the tenants table has no RLS enabled — it IS the
	// tenant boundary); user_count + group_count are computed via
	// correlated subqueries on the users / location_groups tables under
	// `SET LOCAL row_security = off` as a fail-loud guard so a future
	// loss of the connection role's RLS bypass produces a 5xx rather
	// than a silently empty page. Memory just walks the in-memory stores.
	//
	// Total is the count before LIMIT/OFFSET is applied.
	ListAdmin(ctx context.Context, opts AdminTenantListOptions) (items []*AdminTenantListItem, total int, err error)

	// GetAdmin returns a single tenant row with the same computed counts
	// the listing surfaces (user_count, group_count). Used by the
	// `/api/v1/admin/tenants/{tenantID}` detail handler — sharing the
	// SET LOCAL row_security = off fail-loud guard with ListAdmin keeps
	// the count semantics identical between list and detail, and avoids
	// the previous "load every row and call len()" implementation that
	// scaled linearly with tenant size. Returns ErrNotFound when the
	// tenant id doesn't exist.
	GetAdmin(ctx context.Context, tenantID string) (*AdminTenantListItem, error)
}

// AdminTenantSortField names the columns the admin tenant listing
// endpoint understands for sorting. Names are part of the public API
// surface; the FE codegen treats them as opaque strings sent in `?sort`.
type AdminTenantSortField string

const (
	AdminTenantSortName      AdminTenantSortField = "name"
	AdminTenantSortSlug      AdminTenantSortField = "slug"
	AdminTenantSortCreatedAt AdminTenantSortField = "created_at"
	AdminTenantSortStatus    AdminTenantSortField = "status"
)

// IsValid reports whether s is a known admin tenant sort field. Callers
// should fall back to AdminTenantSortName on invalid input rather than
// 4xx — the FE may pass an unknown sort during a multi-version rollout.
func (s AdminTenantSortField) IsValid() bool {
	switch s {
	case AdminTenantSortName, AdminTenantSortSlug, AdminTenantSortCreatedAt, AdminTenantSortStatus:
		return true
	}
	return false
}

// AdminTenantListOptions narrows the result of TenantRegistry.ListAdmin.
// Page/PerPage are 1-based; PerPage <= 0 falls back to a sensible default
// at the registry layer, and the handler caps it at 100.
type AdminTenantListOptions struct {
	// Page is the 1-based page index. Defaults to 1 when <= 0.
	Page int
	// PerPage is the requested page size. Defaults to 50 when <= 0.
	PerPage int
	// Query, when non-empty, narrows the result to tenants whose name,
	// slug, or domain ILIKE %query%. The match is case-insensitive and
	// uses substring semantics so operators can search by partial slug
	// or company-name prefix.
	Query string
	// SortField is the column to sort by. Defaults to AdminTenantSortName
	// when empty or invalid.
	SortField AdminTenantSortField
	// SortDesc reverses the natural order of the chosen field. Default
	// is false (ascending) so the FE sends `-name` style strings and the
	// handler splits the leading `-` into this bool.
	SortDesc bool
}

// AdminTenantListItem is the row shape returned by
// TenantRegistry.ListAdmin: the tenant row plus the computed cross-table
// counts the admin listing UI needs to render at a glance.
type AdminTenantListItem struct {
	Tenant     *models.Tenant
	UserCount  int
	GroupCount int
}

// BackofficeUserRegistry persists platform-operator identities used by
// the back-office auth plane (issue #1785). Unlike UserRegistry, this
// table has NO tenant_id and NO RLS — back-office users live OUTSIDE
// the tenant model and are gated entirely at the application layer.
//
// Phase 1 surfaces only the storage primitives the bootstrap CLI needs;
// later phases add the HTTP login flow + role-aware authorisation on top
// of these methods. Two design choices worth pinning:
//
//  1. GetByEmail is case-insensitive. The registry layer lowercases the
//     email on every read and every write, and the postgres UNIQUE INDEX
//     is defined over the column (not lower(email), which the migrator
//     annotations cannot express). Bypass paths that INSERT a mixed-case
//     email would let duplicates through, so callers MUST go through
//     this interface for create/update/lookup.
//
//  2. SetPasswordHash and UpdateLastLogin are isolated from Update so
//     the bcrypt hash never gets exposed through a generic Update path
//     that the future HTTP layer might call with a partially populated
//     struct. The same shape worked for refresh tokens (RevokeByID /
//     UpdateLastUsedAt) and is the cheapest way to keep the write-path
//     surface explicit.
type BackofficeUserRegistry interface {
	Registry[models.BackofficeUser]

	// GetByEmail returns the back-office user whose lowercased email
	// matches the lowercased argument. Returns ErrBackofficeUserNotFound
	// when no row exists.
	GetByEmail(ctx context.Context, email string) (*models.BackofficeUser, error)

	// SetPasswordHash overwrites only the password_hash column on the
	// target row, leaving every other field untouched. Keeps the bcrypt
	// hash out of any generic Update call site.
	SetPasswordHash(ctx context.Context, id, hash string) error

	// UpdateLastLogin stamps last_login_at on the target row. Called by
	// the Phase 2 login flow on each successful authentication.
	UpdateLastLogin(ctx context.Context, id string, at time.Time) error

	// SetActive flips is_active to the requested value. Used by the
	// future back-office admin UI; in Phase 1 only the bootstrap CLI
	// exercises this indirectly via the initial-user create path.
	SetActive(ctx context.Context, id string, active bool) error
}

// BackofficeRefreshTokenRegistry persists long-lived refresh tokens
// for the back-office auth plane (issue #1785, Phase 2). It is a NARROW
// surface tailored to the back-office plane's actual call sites — it
// does NOT mirror the tenant-side RefreshTokenRegistry method-for-method
// (the tenant interface exposes `GetByUserID`, `RevokeAllExceptID`, and
// other surface area the back-office plane doesn't currently need; the
// back-office plane intentionally narrows to only what its handlers
// require so the misuse surface stays small).
//
// The table has NO row-level security (same reasoning as
// `backoffice_users`: it lives OUTSIDE the tenant model). The login flow
// must be able to look up a row before any DB session context is set,
// so RLS predicates that read `get_current_*_id()` would block the
// very call that needs to authenticate.
type BackofficeRefreshTokenRegistry interface {
	Registry[models.BackofficeRefreshToken]

	// GetByHash returns the refresh-token row whose token_hash matches.
	// Used by the refresh handler (cookie value -> SHA-256 -> row). Returns
	// ErrBackofficeRefreshTokenNotFound when no row exists.
	GetByHash(ctx context.Context, tokenHash string) (*models.BackofficeRefreshToken, error)

	// Revoke marks a single token row as revoked by id, gated on the
	// supplied backofficeUserID so a stolen id can't be used to revoke
	// a session that belongs to a different back-office user. Idempotent:
	// re-revoking an already-revoked row is a no-op success.
	Revoke(ctx context.Context, backofficeUserID, id string) error

	// BumpLastUsedAt sets the last_used_at column on a single token row
	// to `at`, gated on the supplied (backofficeUserID, id) pair so a
	// stolen id from a different operator can't be used to rewrite
	// someone else's row. This is the ONLY field-level mutation the
	// back-office refresh flow needs: the deliberately narrow surface
	// replaces a generic Update method and forecloses the
	// "stolen-id-rewrite" primitive a wide-open Update would expose
	// (cf. the same pattern in BackofficeUserRegistry.SetPasswordHash).
	// Idempotent: bumping a missing or already-revoked row returns
	// ErrBackofficeRefreshTokenNotFound; bumping an existing row to the
	// same timestamp is allowed (no-op cost).
	BumpLastUsedAt(ctx context.Context, backofficeUserID, id string, at time.Time) error

	// ListActiveByBackofficeUserID returns the non-revoked, non-expired
	// rows for the given back-office user ordered most-recently-used
	// first (LastUsedAt desc, CreatedAt desc tiebreaker). Mirrors the
	// equivalent on RefreshTokenRegistry.
	ListActiveByBackofficeUserID(ctx context.Context, backofficeUserID string) ([]*models.BackofficeRefreshToken, error)

	// RevokeByBackofficeUserID marks every refresh-token row for the
	// given back-office user as revoked. Used on password change / forced
	// logout-all scenarios.
	RevokeByBackofficeUserID(ctx context.Context, backofficeUserID string) error

	// DeleteExpired removes all rows whose expires_at is in the past.
	// Called by the retention sweep (future worker). Returning no count
	// keeps the surface narrow — callers only care about success/error.
	DeleteExpired(ctx context.Context) error
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

// CommodityScanAuditRegistry records each invocation of the AI vision
// scan endpoint (#1720). It is the source of truth for both
// observability (cost/usage dashboards) and the in-DB rate limiter
// — the service counts recent rows for the requesting user to gate a
// new scan. The registry runs in service mode (the writer record path
// crosses an RLS boundary when the deadline has fired and the request
// context is no longer usable; cleaner to bypass RLS for the audit
// row itself and rely on the application carrying tenant/user
// identifiers on the entity).
type CommodityScanAuditRegistry interface {
	// Record persists a scan audit row. The implementation generates
	// ID/UUID and sets CreatedAt. The returned pointer is the stored
	// row.
	Record(ctx context.Context, audit models.CommodityScanAudit) (*models.CommodityScanAudit, error)

	// CountRecentForUser returns the number of rows for (tenantID,
	// userID) created at or after the given cutoff. Used by the
	// per-user rate limiter in CommodityScanService.
	//
	// The interface contract is explicit-tenant for parity with the
	// memory implementation, which has no RLS to lean on. The postgres
	// implementation can additionally rely on RLS once a user-scoped
	// registry is used, but the explicit tenant_id predicate stays in
	// the query so the caller doesn't have to reason about which mode
	// the registry was constructed in.
	CountRecentForUser(ctx context.Context, tenantID, userID string, since time.Time) (int, error)

	// DeleteOlderThan removes audit rows older than cutoff. Run by the
	// retention worker (future) — not yet wired but the entry point
	// belongs to this registry, not the worker.
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

	// MarkVerified atomically claims the verification token by setting
	// verified_at from NULL to the current time. The UPDATE filter includes
	// `verified_at IS NULL`, so exactly one of N concurrent requests carrying
	// the same token wins the row: it returns (true, nil) and is the caller
	// responsible for the one-time first-verification side effects (welcome
	// email, audit log, ...). Every other caller — whether it lost the race
	// or the token was already verified earlier — gets (false, nil) and must
	// treat the verification as already done. A token that does not exist
	// also yields (false, nil). This closes the check-then-act race between
	// the IsVerified() read and the write that previously let two requests
	// both run the first-verification side effects (#1005).
	MarkVerified(ctx context.Context, token string) (bool, error)

	// DeleteExpired removes all records whose expiry time has passed.
	DeleteExpired(ctx context.Context) error
}

// MagicLinkTokenRegistry manages passwordless sign-in ("magic link") tokens.
type MagicLinkTokenRegistry interface {
	Registry[models.MagicLinkToken]

	// GetByToken returns a magic-link token record by its token value.
	GetByToken(ctx context.Context, token string) (*models.MagicLinkToken, error)

	// DeleteByUserID removes all magic-link token records for the given user.
	DeleteByUserID(ctx context.Context, userID string) error

	// MarkClaimed atomically claims the sign-in token by setting
	// claimed_at from NULL to the current time. The UPDATE filter includes
	// `claimed_at IS NULL` AND `expires_at > now`, so exactly one of N concurrent
	// requests carrying the same token wins the row: it returns (true, nil) and is
	// the caller responsible for the one-time sign-in side effects (mint a
	// session). Every other caller — whether it lost the race or the token was
	// already claimed earlier — gets (false, nil) and must treat the link as
	// already consumed. A token that does not exist, or one that has expired,
	// also yields (false, nil): folding the expiry check into the UPDATE means an
	// expired token can never be burned. This closes the check-then-act race that
	// would otherwise let two requests both complete sign-in from the same link.
	MarkClaimed(ctx context.Context, token string) (bool, error)

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

	// HasSentInRange reports whether a reminder row that is not silent
	// exists for the commodity with fromDays <= threshold < belowDays. The
	// worker uses it to skip a lead time once an email for a closer one
	// has gone out (#2805). Silent rows do not count: no email went out
	// for them.
	HasSentInRange(ctx context.Context, commodityID string, fromDays, belowDays int) (bool, error)

	// CreateOnce attempts to insert the reminder row. Returns
	// (true, nil) if this call won the insert and the caller may
	// proceed to send the email. Returns (false, nil) when a row for
	// the same tuple already exists (idempotency: another tick or
	// process beat us). Other errors are returned as-is.
	CreateOnce(ctx context.Context, reminder models.WarrantyReminder) (bool, error)
}

// StorageQuotaReminderRegistry is the worker-only registry that
// records "storage quota warning at threshold X has been emitted for
// group Y" rows (#1585). The (group_id, threshold_percent) tuple is
// unique — Create returns (false, nil) for the loser of a race so the
// worker can treat the happy path and the race-loser path identically
// (both mean "no email is needed from this tick").
//
// Reset semantics: the worker calls DeleteByGroupThreshold whenever a
// group's usage drops back below the threshold so the next re-cross
// fires a fresh email. There is no user-facing surface on this table.
//
// All operations run under the background-worker RLS bypass.
// WeeklyDigestSendRegistry is the idempotency store for the weekly digest
// worker (#1391). The digest is per user and covers a calendar week, so the
// (user_id, week_start) pair is what a send is recorded against.
type WeeklyDigestSendRegistry interface {
	// ClaimWeek records that this user's week has been sent and reports
	// whether this call is the one that recorded it. A false answer means
	// somebody else got there first and the caller must not send.
	//
	// Claiming before sending rather than after is deliberate: for a weekly
	// summary a missed digest is cheaper than a duplicate one. See
	// models.WeeklyDigestSend.
	ClaimWeek(ctx context.Context, send models.WeeklyDigestSend) (bool, error)

	// ReleaseWeek drops one user's claim on a week, so a send that failed after
	// the claim can be retried on the next tick instead of losing the week.
	ReleaseWeek(ctx context.Context, userID string, weekStart time.Time) error

	// DeleteSentBefore removes claims for weeks starting before weekStart and
	// returns how many went. The rows only exist to stop a second send inside
	// one week.
	DeleteSentBefore(ctx context.Context, weekStart time.Time) (int, error)
}

type StorageQuotaReminderRegistry interface {
	// HasSent reports whether a reminder row already exists for the
	// given (group, threshold) tuple. Used by the worker to skip the
	// email-render path when the row is present.
	HasSent(ctx context.Context, groupID string, thresholdPercent int) (bool, error)

	// CreateOnce attempts to insert the reminder row. Returns
	// (true, nil) if this call won the insert and the caller may
	// proceed to send the email. Returns (false, nil) when a row for
	// the same tuple already exists (idempotency: another tick or
	// process beat us). Other errors are returned as-is.
	CreateOnce(ctx context.Context, reminder models.StorageQuotaReminder) (bool, error)

	// DeleteByGroupThreshold removes the reminder row for the given
	// (group, threshold) tuple, returning true when a row was actually
	// deleted. Called by the worker when a group drops back below the
	// threshold so a future re-crossing fires a fresh email.
	DeleteByGroupThreshold(ctx context.Context, groupID string, thresholdPercent int) (bool, error)
}

// LocationGroupRegistry manages location groups within a tenant.
// Groups are tenant-scoped (not user-scoped) — access is controlled via memberships.
type LocationGroupRegistry interface {
	Registry[models.LocationGroup]

	// GetBySlug returns a group by its slug within a tenant.
	GetBySlug(ctx context.Context, tenantID, slug string) (*models.LocationGroup, error)

	// ListByTenant returns all groups for a tenant.
	ListByTenant(ctx context.Context, tenantID string) ([]*models.LocationGroup, error)

	// ListAdmin returns a paginated, filtered, and sorted listing of every
	// location group in the deployment alongside per-group computed
	// member_count (accepted memberships only) and the owning tenant. The
	// endpoint behind this method (/api/v1/admin/groups — #1748) crosses
	// tenants by design, so implementations MUST return rows across all
	// tenants regardless of caller membership. The postgres
	// LocationGroupRegistry runs the cross-tenant read as the background-
	// worker role (which carries RLS bypass policies); `SET LOCAL
	// row_security = off` is added as defense-in-depth to make the cross-
	// tenant read explicit, not as the primary guard. member_count is
	// computed via a correlated subquery on group_memberships, and the
	// owning tenant is resolved per row (postgres JOINs tenants in the same
	// tx; memory looks it up via the tenant registry) so the FE renders an
	// owning-tenant chip per row without an N+1 lookup. Memory walks the
	// in-memory stores.
	//
	// Total is the count before LIMIT/OFFSET is applied.
	ListAdmin(ctx context.Context, opts AdminGroupListOptions) (items []*AdminGroupListItem, total int, err error)

	// GetAdmin returns a single group detail row with the same computed
	// member_count the listing surfaces, joined with the owning tenant so
	// the detail handler can render the tenant chip without a second
	// round-trip. Used by the `/api/v1/admin/groups/{groupID}` detail
	// handler. Returns ErrNotFound when the group id doesn't exist.
	GetAdmin(ctx context.Context, groupID string) (*AdminGroupDetail, error)

	// MarkPendingDeletionAdmin flips a group's status to pending_deletion
	// for the cross-tenant admin soft-delete (/api/v1/admin/groups/{groupID}
	// DELETE — #1748). It bypasses RLS so a system admin can act on any
	// tenant's group; the status-transition logic is identical to
	// GroupService.InitiateGroupDeletion (Status = pending_deletion,
	// UpdatedAt bumped) so the existing group_purge_worker finishes the
	// hard-delete with no parallel code path.
	//
	// It returns the post-transition detail row (the same shape GetAdmin
	// returns: group row + member_count + tenant chip) computed inside the
	// SAME transaction as the status write, so the handler renders directly
	// from it with NO second round-trip. Without this the handler would have
	// to re-fetch via GetAdmin, which races the group_purge_worker — between
	// the soft-delete commit and the re-fetch the worker can hard-delete the
	// now-pending row, turning a DELETE that actually succeeded into a 404.
	//
	// The call is idempotent: when the group is already pending_deletion it
	// returns (item, alreadyPending=true, nil) without re-writing the row so
	// the handler can render a 200 with the current status. On a genuine
	// transition it returns (item, false, nil); on a missing group it returns
	// (nil, false, ErrNotFound).
	MarkPendingDeletionAdmin(ctx context.Context, groupID string) (item *AdminGroupDetail, alreadyPending bool, err error)

	// SetHiddenSections replaces the sections an active group hides and
	// sets updated_at, writing those two columns only, so a rename or a
	// deletion that commits meanwhile is kept. The caller passes a
	// validated, normalized list. It returns the stored group and
	// applied=true. A group that is not active is left as it is: the call
	// returns the group with applied=false. A missing group gives
	// ErrNotFound.
	SetHiddenSections(ctx context.Context, groupID string, sections models.GroupSections, updatedAt time.Time) (group *models.LocationGroup, applied bool, err error)

	// UpdateDetails writes the name, icon and description of an active
	// group, its hidden sections when details carries them, and updated_at.
	// No other column is written, so a deletion, a currency migration or a
	// settings change that commits meanwhile is kept. The name is required.
	// It returns the stored group and applied=true. A group that is not
	// active is left as it is: the call returns the group with
	// applied=false. A missing group gives ErrNotFound.
	//
	// The generic Update writes every column from the value it is given;
	// the API's write paths use UpdateDetails, SetHiddenSections,
	// MarkPendingDeletion and SetCurrencyMigrationID instead.
	UpdateDetails(ctx context.Context, groupID string, details GroupDetails, updatedAt time.Time) (group *models.LocationGroup, applied bool, err error)

	// MarkPendingDeletion moves an active group to pending_deletion and sets
	// updated_at, writing those two columns only; the group purge worker
	// deletes the group later. It returns the stored group and
	// applied=true. A group that is not active is left as it is: the call
	// returns the group with applied=false. A missing group gives
	// ErrNotFound. MarkPendingDeletionAdmin is the cross-tenant form the
	// back office uses.
	MarkPendingDeletion(ctx context.Context, groupID string, updatedAt time.Time) (group *models.LocationGroup, applied bool, err error)

	// SetCurrencyMigrationID records the currency migration that holds the
	// group's item write lock, writing currency_migration_id only. It writes
	// nothing and returns applied=false when the group is not active, or
	// when the migration is no longer pending or running: the worker that
	// finished it has already cleared the id, and writing the id back would
	// show the group as migrating until another migration replaced it. The
	// memory backend runs no migrations, so it checks the group alone. A
	// missing group gives ErrNotFound.
	SetCurrencyMigrationID(ctx context.Context, groupID, migrationID string) (applied bool, err error)
}

// GroupDetails is what LocationGroupRegistry.UpdateDetails writes: the name,
// icon and description, which are always written, and the hidden sections,
// which are written only when HiddenSections is not nil.
type GroupDetails struct {
	Name           string
	Icon           string
	Description    string
	HiddenSections *models.GroupSections
}

// AdminGroupSortField names the columns the admin group listing endpoint
// understands for sorting. Names are part of the public API surface; the
// FE codegen treats them as opaque strings sent in `?sort`.
type AdminGroupSortField string

const (
	AdminGroupSortName      AdminGroupSortField = "name"
	AdminGroupSortSlug      AdminGroupSortField = "slug"
	AdminGroupSortCreatedAt AdminGroupSortField = "created_at"
	AdminGroupSortStatus    AdminGroupSortField = "status"
)

// IsValid reports whether s is a known admin group sort field. Callers
// should fall back to AdminGroupSortName on invalid input rather than
// 4xx — the FE may pass an unknown sort during a multi-version rollout.
func (s AdminGroupSortField) IsValid() bool {
	switch s {
	case AdminGroupSortName, AdminGroupSortSlug, AdminGroupSortCreatedAt, AdminGroupSortStatus:
		return true
	}
	return false
}

// AdminGroupListOptions narrows the result of LocationGroupRegistry.ListAdmin.
// Page/PerPage are 1-based; PerPage <= 0 falls back to a sensible default
// at the registry layer, and the handler caps it at 100.
type AdminGroupListOptions struct {
	// Page is the 1-based page index. Defaults to 1 when <= 0.
	Page int
	// PerPage is the requested page size. Defaults to 50 when <= 0.
	PerPage int
	// Query, when non-empty, narrows the result to groups whose name or
	// slug ILIKE %query%. The match is case-insensitive and uses
	// substring semantics.
	Query string
	// TenantID, when non-empty, narrows the result to groups belonging to
	// that exact tenant.
	TenantID string
	// Status, when non-empty, narrows the result to groups in that exact
	// status (active / pending_deletion).
	Status string
	// SortField is the column to sort by. Defaults to AdminGroupSortName
	// when empty or invalid.
	SortField AdminGroupSortField
	// SortDesc reverses the natural order of the chosen field. Default
	// is false (ascending).
	SortDesc bool
}

// AdminGroupListItem is the row shape returned by
// LocationGroupRegistry.ListAdmin: the group row plus the computed
// member_count and the owning tenant the admin listing UI needs to
// render at a glance. Tenant is resolved per row (joined / looked up by
// the registry) so the cross-tenant admin list can render an owning-
// tenant chip without an FE N+1 lookup. Tenant may be nil if the join
// row is somehow missing, but a group with a non-NULL tenant_id FK
// should always resolve one.
type AdminGroupListItem struct {
	Group       *models.LocationGroup
	MemberCount int
	Tenant      *models.Tenant
}

// AdminGroupDetail is the row shape returned by
// LocationGroupRegistry.GetAdmin: the group row plus the computed
// member_count and the owning tenant (resolved for the detail-page
// tenant chip). Tenant may be nil if the join row is somehow missing,
// but a group with a non-NULL tenant_id FK should always resolve one.
type AdminGroupDetail struct {
	Group       *models.LocationGroup
	MemberCount int
	Tenant      *models.Tenant
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

	// ListByGroupWithUsersAdmin is the cross-tenant twin of
	// ListByGroupWithUsers, backing the #1756 admin membership editor.
	// A system administrator is not tenant-scoped, so the postgres
	// backend runs the join under `SET LOCAL row_security = off` — the
	// same defense-in-depth RLS bypass LocationGroupRegistry.GetAdmin /
	// ListAdmin use — so a group in ANY tenant lists fine. The memory
	// backend has no RLS and simply delegates to the same join logic.
	ListByGroupWithUsersAdmin(ctx context.Context, groupID string) ([]*models.MembershipWithUser, error)

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

	// DeleteByGroup removes every per-user notification override for the
	// given (tenant, group). Used by the group-deletion cleanup path to
	// break the group_notification_prefs.group_id -> location_groups(id)
	// FK (NO ACTION) before the group is removed. Returns the number of
	// rows deleted. Idempotent: zero matches returns (0, nil).
	DeleteByGroup(ctx context.Context, tenantID, groupID string) (int, error)
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

	// ListAdminByTenant returns a paginated, filtered, and sorted listing
	// of every user in the given tenant alongside per-row group membership
	// counts. The endpoint behind this method
	// (/api/v1/admin/tenants/{tenantID}/users — #1746) crosses tenants by
	// design, so implementations MUST return rows for the requested
	// tenant regardless of caller membership. The postgres UserRegistry
	// uses NonRLSRepository for the cross-tenant read and `SET LOCAL
	// row_security = off` on the join tx as a fail-loud guard (a future
	// loss of the connection role's RLS bypass surfaces as 5xx, not a
	// silently empty page). Total is the count before LIMIT/OFFSET.
	ListAdminByTenant(ctx context.Context, tenantID string, opts AdminUserListOptions) (items []*AdminUserListItem, total int, err error)

	// CountSessionsByUser returns the number of unrevoked, unexpired
	// refresh_tokens rows belonging to the user. Backs the
	// `active_session_count` field on the admin user-detail endpoint
	// (#1746). Implementations cross tenants intentionally so the admin
	// surface can see sessions for users in any tenant.
	//
	// The admin handler degrades a CountSessionsByUser failure to 0 +
	// a secondary `admin.get_user_sessions` audit row rather than 500-ing
	// the whole user-detail endpoint, so audit consumers must correlate
	// by ActorID + timestamp to distinguish "genuine 0 sessions" from
	// "session-count registry hiccup".
	CountSessionsByUser(ctx context.Context, userID string) (int, error)
}

// SystemAdminGrantRegistry stores the dedicated grant rows that confer
// platform-wide system-admin privilege on a user (#1784). Splitting the
// privilege off the users row removes the escalation footgun of a
// "just UPDATE users SET is_system_admin = true" path.
//
// Write-surface invariant: no production HTTP handler can mutate this
// table. Production write paths are the `inventario admin` CLI
// (grant-system-admin / revoke-system-admin) only. The lone exception
// is the debug seed flow at POST /api/v1/seed, which can mint a grant
// via ensureSystemAdminUser — but only the system-admin fixture/grant
// path within that handler is gated on INVENTARIO_SEED_SYSTEM_ADMIN_FIXTURE
// (off by default, never set in production deployments; the e2e harness
// uses it, nothing else does). The seed endpoint itself is mounted
// unconditionally; the env var controls only whether a sysadmin grant
// is created during seeding.
//
// The registry is NOT tenant-scoped: system-admin is a platform privilege
// orthogonal to tenants. Same posture as AuditLogRegistry.
type SystemAdminGrantRegistry interface {
	// Exists returns true when the user has a grant row. Hot path —
	// Read by the /auth/me advisory flag and by impersonationTargetGuard.
	// Postgres backs the lookup with a unique index on user_id; memory
	// keeps a simple map keyed by user id.
	Exists(ctx context.Context, userID string) (bool, error)

	// Grant inserts a grant row. Idempotent: when the user is already
	// a system admin, returns (true, nil) and does not mutate the row.
	// grantedBy is the operator who authorised the grant; nil for CLI
	// bootstrap (no authenticated session).
	Grant(ctx context.Context, userID string, grantedBy *string) (hadGrant bool, err error)

	// RevokeAtomic removes the grant row while serializing against
	// concurrent revokes. With allowZero=false, enforces the "at least
	// one grant remains" invariant — returns ErrLastSystemAdmin
	// otherwise. Idempotent: when the user has no grant, returns
	// (false, nil) with no row touched. The postgres impl serialises
	// via pg_advisory_xact_lock('system_admin_mutations') — the same
	// lock key the legacy users.is_system_admin path used — so a
	// rolling deploy is race-free even mid-cutover. allowZero=true
	// bypasses the guard; exposed on the CLI as --allow-zero only.
	//
	//revive:disable-next-line:flag-parameter
	RevokeAtomic(ctx context.Context, userID string, allowZero bool) (hadGrant bool, err error)

	// List returns every grant row, ordered by (granted_at ASC,
	// user_id ASC). The user_id secondary key keeps iteration order
	// stable when two grants share a granted_at — fast-fired CLI
	// grants can tie on `now()` resolution otherwise, which would
	// shuffle the rendered list across reads. Backs the
	// `inventario admin list-system-admins` CLI command (the CLI
	// joins to users for the rendered table).
	List(ctx context.Context) ([]*models.SystemAdminGrant, error)
}

// WorkerControlRegistry stores the global soft-pause control rows for
// background workers (#1308). One row per worker type; an absent row
// means the worker runs normally, a present row with paused=true means
// the worker's run loop skips its work each tick until resumed.
//
// The registry is NOT tenant-scoped and has NO RLS — worker pause state
// is a platform-operator control orthogonal to tenants (same posture as
// SystemAdminGrantRegistry / AuditLogRegistry). It lives directly on
// FactorySet and is identical in user-mode and service-mode sets.
type WorkerControlRegistry interface {
	// List returns every worker_control row. An absent worker type means
	// that worker is running (the caller treats "no row" as not-paused).
	List(ctx context.Context) ([]*models.WorkerControl, error)

	// Pause idempotently marks workerType paused. pausedBy and reason may
	// be "" (stored as NULL). Re-pausing an already-paused type updates
	// paused_by/reason but PRESERVES the original paused_at so the pause
	// timestamp reflects when the worker first stopped. Returns the
	// resulting row.
	Pause(ctx context.Context, workerType, pausedBy, reason string) (*models.WorkerControl, error)

	// Resume idempotently marks workerType not paused, clearing
	// paused_at/paused_by/reason. When no row exists it is a no-op and
	// returns a synthetic not-paused WorkerControl{WorkerType: workerType}
	// without inserting a row. Returns the resulting state.
	Resume(ctx context.Context, workerType string) (*models.WorkerControl, error)
}

// AdminUserSortField names the columns the admin user listing endpoint
// understands for sorting. Names are part of the public API surface; the
// FE codegen treats them as opaque strings sent in `?sort`.
type AdminUserSortField string

const (
	AdminUserSortEmail       AdminUserSortField = "email"
	AdminUserSortName        AdminUserSortField = "name"
	AdminUserSortCreatedAt   AdminUserSortField = "created_at"
	AdminUserSortLastLoginAt AdminUserSortField = "last_login_at"
	AdminUserSortIsActive    AdminUserSortField = "is_active"
)

// IsValid reports whether s is a known admin user sort field. Callers
// should fall back to AdminUserSortEmail on invalid input rather than
// 4xx — the FE may pass an unknown sort during a multi-version rollout.
func (s AdminUserSortField) IsValid() bool {
	switch s {
	case AdminUserSortEmail, AdminUserSortName, AdminUserSortCreatedAt, AdminUserSortLastLoginAt, AdminUserSortIsActive:
		return true
	}
	return false
}

// AdminUserListOptions narrows the result of UserRegistry.ListAdminByTenant.
// Page/PerPage are 1-based; PerPage <= 0 falls back to a sensible default
// at the registry layer, and the handler caps it at 100.
type AdminUserListOptions struct {
	// Page is the 1-based page index. Defaults to 1 when <= 0.
	Page int
	// PerPage is the requested page size. Defaults to 50 when <= 0.
	PerPage int
	// Query, when non-empty, narrows the result to users whose email or
	// name ILIKE %query%. Case-insensitive substring match.
	Query string
	// IsActive is tri-state. Nil means "no filter"; a non-nil value
	// restricts to rows whose is_active column equals *IsActive.
	IsActive *bool
	// SortField is the column to sort by. Defaults to AdminUserSortEmail
	// when empty or invalid.
	SortField AdminUserSortField
	// SortDesc reverses the natural order of the chosen field.
	SortDesc bool
}

// AdminUserListItem is the row shape returned by
// UserRegistry.ListAdminByTenant: the user row plus the computed group
// membership count the admin listing UI surfaces at a glance.
type AdminUserListItem struct {
	User                 *models.User
	GroupMembershipCount int
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

// APITokenRegistry stores personal API tokens (#2808). It runs in service mode:
// token authentication looks a row up by digest before any user context
// exists. The surface is deliberately narrow — there is no generic Update, so
// no caller can rewrite a token's scope, group or owner after creation.
type APITokenRegistry interface {
	// Create inserts a token. ID, UUID and CreatedAt are assigned here.
	Create(ctx context.Context, token models.APIToken) (*models.APIToken, error)

	// GetByTokenHash returns the token whose digest matches, revoked or not.
	// Returns ErrNotFound when no row matches.
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.APIToken, error)

	// ListByUserID returns the user's tokens that are not revoked, expired
	// ones included so the user can see them, newest first.
	ListByUserID(ctx context.Context, userID string) ([]*models.APIToken, error)

	// CountActiveByUserID counts the user's tokens that are neither revoked
	// nor expired at now.
	CountActiveByUserID(ctx context.Context, userID string, now time.Time) (int, error)

	// RevokeByID revokes one token, gated on the owner so a guessed id from
	// another account returns ErrNotFound. Revoking an already revoked token
	// is a no-op success. Returns the token as stored after the call.
	RevokeByID(ctx context.Context, userID, id string) (*models.APIToken, error)

	// RevokeAllByUserID revokes every token of the user that is not revoked
	// yet, and returns how many it revoked.
	RevokeAllByUserID(ctx context.Context, userID string) (int, error)

	// TouchLastUsed sets last_used_at to at, but only when the stored value is
	// NULL or older than staleBefore and the token is not revoked. It reports
	// whether it wrote, so concurrent requests on several replicas produce one
	// write per interval instead of one each.
	TouchLastUsed(ctx context.Context, id string, at, staleBefore time.Time) (bool, error)
}

// PersonalDataExportRegistry stores personal data exports (#2177). It runs in
// service mode: the export worker builds and expires the exports of every
// user. Each method that reads one user's rows takes the user and filters by
// it, so a guessed id of another user's export answers ErrNotFound.
type PersonalDataExportRegistry interface {
	// CreateUnlessRequestedSince inserts export unless the user made a request
	// after since. It returns the inserted row, or nil and the user's
	// newest request when that request is what stops the insert. The check
	// and the insert are one step: two concurrent requests insert one row.
	// ID, UUID, CreatedAt and Status are assigned here.
	CreateUnlessRequestedSince(ctx context.Context, export models.PersonalDataExport, since time.Time) (created, blocking *models.PersonalDataExport, err error)

	// Get returns an export by id, whoever owns it. Only the worker calls it.
	Get(ctx context.Context, id string) (*models.PersonalDataExport, error)

	// GetByUser returns the user's export by id, ErrNotFound for an id that
	// is not one of theirs.
	GetByUser(ctx context.Context, userID, id string) (*models.PersonalDataExport, error)

	// ListByUserID returns the user's exports, newest first.
	ListByUserID(ctx context.Context, userID string) ([]*models.PersonalDataExport, error)

	// ListPending returns the pending exports of every user, oldest first.
	ListPending(ctx context.Context) ([]*models.PersonalDataExport, error)

	// ClaimPending moves an export from pending to in_progress, records
	// startedAt as the start of its build, and reports whether this caller
	// is the one that moved it. The condition is part of the write, so two
	// workers racing for one row get one true.
	ClaimPending(ctx context.Context, id string, startedAt time.Time) (bool, error)

	// RequeueStalled moves the exports whose build started before
	// startedBefore and is still in_progress back to pending, and returns how
	// many it moved. A build whose process dies records no result, and its
	// row would otherwise stay in_progress until it expires.
	RequeueStalled(ctx context.Context, startedBefore time.Time) (int, error)

	// Update writes the status, start, archive and statistics columns of an
	// existing export. The owner, tenant and creation time never change.
	Update(ctx context.Context, export models.PersonalDataExport) (*models.PersonalDataExport, error)

	// ListExpired returns up to limit exports whose expiry is at or before
	// now, oldest expiry first.
	ListExpired(ctx context.Context, now time.Time, limit int) ([]*models.PersonalDataExport, error)

	// Delete removes one export row. Deleting a row that is gone is not an
	// error.
	Delete(ctx context.Context, id string) error
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

// OAuthIdentityRegistry stores links between Inventario users and external
// OAuth provider accounts (#1394). Implementations run in service mode (RLS
// bypass) because the OAuth callback resolves the row by
// (provider, provider_user_id) BEFORE any user session exists — the
// background-worker RLS policy handles the read path; the SQL adds
// `tenant_id = $tenantID` (and equivalents) as defense-in-depth on every
// method that takes a tenantID.
type OAuthIdentityRegistry interface {
	Registry[models.OAuthIdentity]

	// GetByProviderSubject returns the row keyed by (provider, providerUserID).
	// The pair is globally unique, so no tenant qualifier is needed here.
	// Returns ErrNotFound when no row matches — the callback uses this
	// outcome to decide between "log in existing user" and "find by email
	// or create".
	GetByProviderSubject(ctx context.Context, provider models.OAuthProvider, providerUserID string) (*models.OAuthIdentity, error)

	// ListByUser returns every identity linked to userID, scoped to tenantID
	// as defense-in-depth (the RLS policy already enforces tenant + user
	// isolation when the read runs in user mode). Ordering: provider asc
	// so the UI list reads stable.
	ListByUser(ctx context.Context, tenantID, userID string) ([]*models.OAuthIdentity, error)

	// GetByUserAndProvider returns the single row keyed by (tenantID, userID,
	// provider), or ErrNotFound. Used by the unlink endpoint and by the
	// "is this provider already linked?" check on the link path.
	GetByUserAndProvider(ctx context.Context, tenantID, userID string, provider models.OAuthProvider) (*models.OAuthIdentity, error)

	// DeleteByUserAndProvider removes the (tenantID, userID, provider) row
	// idempotently. Returns no error if no row exists — the caller has
	// already enforced the "must keep at least one auth method" guard.
	DeleteByUserAndProvider(ctx context.Context, tenantID, userID string, provider models.OAuthProvider) error
}

// UserMFASecretRegistry stores per-user TOTP credentials (#1380 / #1645).
// The (tenant_id, user_id) tuple is unique — at most one row per user.
// Implementations run in service mode (RLS bypass): the registry is
// hit during the login flow before user/tenant RLS context is set,
// so the user-mode flavour cannot be used.
type UserMFASecretRegistry interface {
	Registry[models.UserMFASecret]

	// GetByUser returns the row for (tenantID, userID), or
	// ErrNotFound when the user has never enrolled.
	GetByUser(ctx context.Context, tenantID, userID string) (*models.UserMFASecret, error)

	// DeleteByUser removes the user's MFA row idempotently. Used by
	// the disable flow. Returns no error if no row exists.
	DeleteByUser(ctx context.Context, tenantID, userID string) error

	// ConsumeBackupCodeAtomic atomically removes the matching hash from
	// the user's BackupCodesHashed slice in a single transaction so two
	// concurrent login_mfa requests racing on the same code can never
	// both succeed (#1645 review). The matcher closure receives one
	// stored hash at a time and returns true on a bcrypt match — that
	// keeps bcrypt out of the SQL layer while the row stays write-locked
	// for the duration of the compare. Returns (true, nil) when one
	// hash was consumed, (false, nil) when none matched, and a
	// classified error on infrastructure failure. Updates LastUsedAt
	// to `now` on a successful consumption alongside the slice rewrite.
	ConsumeBackupCodeAtomic(ctx context.Context, tenantID, userID string, now time.Time, matchHash func(hash string) bool) (bool, error)

	// MarkTOTPStepUsedAtomic compare-and-swaps the row's last_used_step
	// to `step`, succeeding only when the stored last_used_step is
	// strictly less than `step`. This is the TOTP replay guard
	// (RFC 6238 §5.2, #2124): a code's time-step is monotonic, so once a
	// step has been accepted, re-presenting the same code (which computes
	// the same step) loses the CAS and is rejected. Two concurrent step-2
	// requests with the same code compute the same step S; only the first
	// UPDATE (last_used_step < S) affects a row — the second affects zero
	// rows and the caller treats it as a replay. Returns (true, nil) when
	// this call won the swap, (false, nil) when it lost (replay), and a
	// classified error on infrastructure failure. Also stamps LastUsedAt
	// and UpdatedAt to `now` on a win, so the TOTP success path needs no
	// separate last-used bump.
	MarkTOTPStepUsedAtomic(ctx context.Context, tenantID, userID string, step int64, now time.Time) (bool, error)

	// UpdateBackupCodes replaces backup_codes_hashed (the regenerate flow),
	// writing ONLY that column + updated_at. last_used_step / last_used_at are
	// owned exclusively by MarkTOTPStepUsedAtomic; a full-row Update here would
	// clobber a concurrent step advance and re-open the replay window (#2124).
	// Returns ErrNotFound if the row is gone.
	UpdateBackupCodes(ctx context.Context, tenantID, userID string, hashes []string, now time.Time) error
}

// BackofficeUserMFASecretRegistry stores per-back-office-user TOTP
// credentials (issue #1785, Phase 4). One row per back-office user is
// enforced by the unique index on backoffice_user_id. The table has NO
// RLS — same reasoning as backoffice_users / backoffice_refresh_tokens:
// back-office identities live OUTSIDE the tenant model and access is
// gated entirely at the application layer.
//
// Unlike UserMFASecretRegistry, the back-office variant has no over-HTTP
// self-enrollment surface — only the operator CLI mints, regenerates,
// and wipes rows. The interface reflects that: there's no Create method
// that takes a partial row; Upsert atomically replaces (or inserts) the
// single row for the given back-office user, and MarkEnabled flips the
// EnabledAt timestamp from CLI setup.
type BackofficeUserMFASecretRegistry interface {
	// Get returns the row for backofficeUserID, or
	// ErrBackofficeMFASecretNotFound when the user has no enrollment.
	Get(ctx context.Context, backofficeUserID string) (*models.BackofficeUserMFASecret, error)

	// Upsert atomically replaces (or inserts) the single row for the
	// given back-office user. The whole replace runs under a single
	// transaction so a partial write (e.g. secret persisted but backup
	// codes failed) is impossible. The caller is responsible for filling
	// SecretEncrypted and BackupCodesHashed; CreatedAt / UpdatedAt are
	// stamped by the registry.
	Upsert(ctx context.Context, secret models.BackofficeUserMFASecret) (*models.BackofficeUserMFASecret, error)

	// Delete removes the back-office user's MFA row idempotently — a
	// missing row is not an error.
	Delete(ctx context.Context, backofficeUserID string) error

	// MarkEnabled stamps EnabledAt to `at` (replacing any prior value)
	// and bumps UpdatedAt. Used by the CLI setup flow to mark a freshly
	// inserted row as active in a single explicit call rather than
	// relying on a separate Upsert with the timestamp prepopulated.
	MarkEnabled(ctx context.Context, backofficeUserID string, at time.Time) error

	// ConsumeBackupCodeAtomic atomically removes the matching hash from
	// the row's BackupCodesHashed slice. Mirrors
	// UserMFASecretRegistry.ConsumeBackupCodeAtomic — two concurrent
	// step-2 login attempts on the same code can never both succeed.
	// Returns (true, nil) when one hash was consumed, (false, nil)
	// when none matched, and a classified error on infrastructure
	// failure. Updates LastUsedAt to `now` on a successful consumption.
	ConsumeBackupCodeAtomic(ctx context.Context, backofficeUserID string, now time.Time, matchHash func(hash string) bool) (bool, error)

	// MarkTOTPStepUsedAtomic compare-and-swaps the row's last_used_step
	// to `step`, succeeding only when the stored last_used_step is
	// strictly less than `step`. The back-office mirror of
	// UserMFASecretRegistry.MarkTOTPStepUsedAtomic — see there for the
	// full replay-guard rationale (RFC 6238 §5.2, #2124). Returns
	// (true, nil) when this call won the swap, (false, nil) when it lost
	// (replay), and a classified error on infrastructure failure. Also
	// stamps LastUsedAt and UpdatedAt to `now` on a win, so the TOTP
	// success path needs no separate BumpLastUsedAt call.
	MarkTOTPStepUsedAtomic(ctx context.Context, backofficeUserID string, step int64, now time.Time) (bool, error)
}

// GroupPurger hard-deletes every row whose group_id references the given
// LocationGroup, in a FK-safe order: restore_steps, restore_operations,
// exports, file_links, files, commodities, areas, locations and finally
// group_memberships. `files` is purged before `commodities` because file links
// reference commodities polymorphically via (entity_type, entity_id) —
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

// TenantPurger hard-deletes every tenant-scoped dependent row belonging to a
// single Tenant, in a FK-safe order, before the tenants row itself is dropped
// by the orchestration layer (#2115). It is the tenant-level analogue of
// GroupPurger: where GroupPurger clears one group's subtree, TenantPurger
// clears the union of every group's data PLUS the tenant-only rows GroupPurger
// never touches (users, settings, refresh_tokens, login_events, the auth-token
// tables, location_groups themselves, …).
//
// The tenants row itself is NOT touched here — the caller (services.admin
// DeleteTenant) drops it after the dependents are gone, exactly as
// GroupPurgeService handles the final location_groups DELETE separately so blob
// cleanup and tenant removal remain explicit at the orchestration layer.
type TenantPurger interface {
	// PurgeTenantDependents deletes all dependent entities for the given
	// tenant. Implementations must be idempotent — a second call on the same
	// tenant after a partial failure must succeed and leave the database in
	// the same state.
	PurgeTenantDependents(ctx context.Context, tenantID string) error
	// CheckTenantPurge reports whether PurgeTenantDependents and the deletion
	// of the tenants row would succeed now, without changing anything. A
	// tenant deletion runs it before removing the tenant's stored files, which
	// cannot be restored when the row purge then fails.
	CheckTenantPurge(ctx context.Context, tenantID string) error
}

// FailedJobScope narrows a failed-jobs query to one tenant, or to one group
// of a tenant (#1307). The zero value reads every tenant: the system-admin
// plane and the metrics use it.
type FailedJobScope struct {
	// TenantID limits the query to one tenant when set.
	TenantID string
	// GroupID limits the query to one group when set. It requires TenantID.
	GroupID string
}

// FailedJobRegistry reads and acts on the jobs that failed for good, over
// each job type's own table (#1307). There is no table of failed jobs: an
// adapter per worker type knows which rows of its table are failed, how a
// failed row is put back in the queue and how it is marked discarded.
//
// A discarded job keeps its row and its error; it only leaves the failed
// list and the counts. Every method filters by the scope in its query, not
// only through row-level security, so a group scope never reaches another
// group's rows whichever database role runs it.
type FailedJobRegistry interface {
	// ListFailed returns the failed jobs of the type in the scope that are
	// not discarded, newest failure first. A limit above zero caps the
	// result.
	ListFailed(ctx context.Context, jobType models.WorkerType, scope FailedJobScope, limit int) ([]*models.FailedJob, error)

	// CountFailed counts the jobs ListFailed would return without a limit.
	CountFailed(ctx context.Context, jobType models.WorkerType, scope FailedJobScope) (int, error)

	// GetFailed returns one failed job that is not discarded. ErrNotFound
	// when the type has no such job in the scope.
	GetFailed(ctx context.Context, jobType models.WorkerType, scope FailedJobScope, id string) (*models.FailedJob, error)

	// Retry puts a failed job back in its queue the way its type runs it
	// again. ErrNotFound when the type has no failed, undiscarded job with
	// the id in the scope; ErrFailedJobNotRetryable for a type whose jobs
	// cannot be run again safely.
	Retry(ctx context.Context, jobType models.WorkerType, scope FailedJobScope, id string, now time.Time) error

	// Discard marks a failed job discarded at now. ErrNotFound when the
	// type has no failed, undiscarded job with the id in the scope.
	Discard(ctx context.Context, jobType models.WorkerType, scope FailedJobScope, id string, now time.Time) error
}

// UserPurger hard-deletes a single user's auth / identity rows (and orphans the
// nullable authorship back-refs) before the users row itself is dropped by the
// orchestration layer (#2116). It exists because a bare DELETE FROM users is
// rejected by the many NO ACTION child FKs pointing at users(id). This includes
// the user's group_invites_audit references (created_by / used_by), which are
// NOT NULL FKs that nothing else clears (#2147).
//
// PRECONDITION: the user must no longer OWN shared content
// (commodities/files/areas/locations/exports/groups/tags/…). Those rows carry
// NOT NULL user_id + NOT NULL created_by columns that cannot be orphaned, so
// this purger deliberately does not touch them; the final DELETE FROM users
// will fail loudly with an FK violation if any remain. The users row itself is
// NOT touched here — the caller (services.admin DeleteUser) drops it after the
// dependents are gone.
type UserPurger interface {
	// PurgeUserDependents deletes all auth/identity dependent rows for the
	// given tenant/user pair. Implementations must be idempotent — a second
	// call on the same user after a partial failure must succeed and leave the
	// database in the same state.
	PurgeUserDependents(ctx context.Context, tenantID, userID string) error
}

// UserContentOwnershipChecker answers, without mutating anything, whether a user
// still OWNS content that survives a self-service account deletion (#2147). It
// is the read-only pre-check the account-deletion orchestration runs BEFORE any
// purge, so the "you still own shared content" rejection aborts cleanly instead
// of being discovered at the final DELETE FROM users (which would leave a
// half-erased account).
//
// "Owned content" means any row that carries a NOT NULL FK to users(id) which
// cannot be orphaned without a schema change:
//   - content authored by the user (created_by_user_id) in a group that is NOT
//     being purged (commodities/files/areas/locations/exports/tags), and
//   - location_groups the user created (created_by) that are NOT being purged.
//
// purgedGroupIDs is the set of PRIVATE group IDs the caller will delete wholesale
// (the user's content there disappears with the group, so it does not count).
// The check is tenant-scoped as defense-in-depth.
type UserContentOwnershipChecker interface {
	// HasRetainedOwnedContent reports whether the user owns any content (or
	// created any location group) that would survive deleting only the
	// purgedGroupIDs groups. It performs no writes. A true result means the
	// account cannot be erased unilaterally; false means the purge sequence is
	// safe to run.
	HasRetainedOwnedContent(ctx context.Context, tenantID, userID string, purgedGroupIDs []string) (bool, error)
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
	SupplyLinkRegistry             SupplyLinkRegistry
	MaintenanceScheduleRegistry    MaintenanceScheduleRegistry
	CommodityReminderRegistry      CommodityReminderRegistry
	CommodityDraftRegistry         CommodityDraftRegistry
	IntakeRegistry                 IntakeRegistry
	IntakeDocumentRegistry         IntakeDocumentRegistry
	SellerRegistry                 SellerRegistry
	ItemTypeRegistry               ItemTypeRegistry
	CustomFieldRegistry            CustomFieldRegistry
	ThumbnailGenerationJobRegistry ThumbnailGenerationJobRegistry
	UserConcurrencySlotRegistry    UserConcurrencySlotRegistry
	OperationSlotRegistry          OperationSlotRegistry
	TenantRegistry                 TenantRegistry
	UserRegistry                   UserRegistry
	RefreshTokenRegistry           RefreshTokenRegistry
	LoginEventRegistry             LoginEventRegistry            // Append-only login attempt audit (#1379); written by login flow + retention worker
	UserMFASecretRegistry          UserMFASecretRegistry         // UserMFASecretRegistry stores per-user TOTP secrets; service-mode (used during pre-RLS login)
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
	StorageQuotaReminderRegistry   StorageQuotaReminderRegistry  // StorageQuotaReminderRegistry is the idempotency store for the storage quota warning worker; service-mode only (#1585)
	WeeklyDigestSendRegistry       WeeklyDigestSendRegistry      // WeeklyDigestSendRegistry is the idempotency store for the weekly digest worker; service-mode only (#1391)
	MaintenanceReminderRegistry    MaintenanceReminderRegistry   // MaintenanceReminderRegistry is the idempotency store for the maintenance reminder worker; service-mode only (#1368)
	CommodityReminderSendRegistry  CommodityReminderSendRegistry // CommodityReminderSendRegistry is the idempotency store for the item reminder worker; service-mode only (#2805)
	CurrencyMigrationRegistry      CurrencyMigrationRegistry     // Currency migration operation rows + audit + HMAC token signing (issue #1550 / epic #202)
	CommodityScanAuditRegistry     CommodityScanAuditRegistry    // CommodityScanAuditRegistry records every AI vision scan request (#1720); also backs the per-user rate limiter
	SystemAdminGrantRegistry       SystemAdminGrantRegistry      // Platform-admin grant rows (#1784); no tenant scope, no HTTP write surface
	OAuthIdentityRegistry          OAuthIdentityRegistry         // OAuth provider link rows (#1394); service-mode (looked up before user session exists in callback)
	WorkerControlRegistry          WorkerControlRegistry         // Background-worker soft-pause control rows (#1308); no tenant scope, no RLS
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
		validation.Field(&s.SupplyLinkRegistry, validation.Required),
		validation.Field(&s.MaintenanceScheduleRegistry, validation.Required),
		validation.Field(&s.CommodityReminderRegistry, validation.Required),
		validation.Field(&s.CommodityDraftRegistry, validation.Required),
		validation.Field(&s.IntakeRegistry, validation.Required),
		validation.Field(&s.IntakeDocumentRegistry, validation.Required),
		validation.Field(&s.SellerRegistry, validation.Required),
		validation.Field(&s.ItemTypeRegistry, validation.Required),
		validation.Field(&s.CustomFieldRegistry, validation.Required),
		validation.Field(&s.TenantRegistry, validation.Required),
		validation.Field(&s.UserRegistry, validation.Required),
		validation.Field(&s.CommodityScanAuditRegistry, validation.Required),
		validation.Field(&s.SystemAdminGrantRegistry, validation.Required),
		validation.Field(&s.OAuthIdentityRegistry, validation.Required),
		validation.Field(&s.WorkerControlRegistry, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, s, fields...)
}
