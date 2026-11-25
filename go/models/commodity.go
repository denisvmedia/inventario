package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models/rules"
)

var (
	_ validation.Validatable = (*CommodityStatus)(nil)
)

type CommodityStatus string

// Commodity statuses. Adding a new status? Don't forget to update IsValid() method.
const (
	CommodityStatusInUse      CommodityStatus = "in_use"
	CommodityStatusSold       CommodityStatus = "sold"
	CommodityStatusLost       CommodityStatus = "lost"
	CommodityStatusDisposed   CommodityStatus = "disposed"
	CommodityStatusWrittenOff CommodityStatus = "written_off"
)

func (c CommodityStatus) IsValid() bool {
	switch c {
	case CommodityStatusInUse,
		CommodityStatusSold,
		CommodityStatusLost,
		CommodityStatusDisposed,
		CommodityStatusWrittenOff:
		return true
	}
	return false
}

func (c CommodityStatus) Validate() error {
	if !c.IsValid() {
		return validation.NewError("invalid_status", "invalid status")
	}
	return nil
}

var (
	_ validation.Validatable = (*CommodityType)(nil)
)

// CommodityType is the slug of one of the item types of the item's group
// (see ItemType). The six built-in types keep the slugs below; a type a
// member adds gets a slug made from its name. Whether a slug names a type of
// the group is checked by the registries, inside the write's transaction;
// the model checks only its shape.
type CommodityType string

// The slugs of the built-in item types, seeded in every group (see
// BuiltinItemTypes).
const (
	CommodityTypeWhiteGoods  CommodityType = "white_goods"
	CommodityTypeElectronics CommodityType = "electronics"
	CommodityTypeEquipment   CommodityType = "equipment"
	CommodityTypeFurniture   CommodityType = "furniture"
	CommodityTypeClothes     CommodityType = "clothes"
	CommodityTypeOther       CommodityType = "other"
)

// IsValid reports whether c has the shape of an item type slug: lowercase
// letters and digits, in runs separated by single hyphens or underscores, at
// most MaxItemTypeSlugRunes runes.
func (c CommodityType) IsValid() bool {
	s := string(c)
	return s != "" &&
		utf8.RuneCountInString(s) <= MaxItemTypeSlugRunes &&
		itemTypeSlugPattern.MatchString(s) &&
		strings.ToLower(s) == s
}

// IsBuiltin reports whether c is the slug of a built-in item type.
func (c CommodityType) IsBuiltin() bool {
	_, ok := builtinItemTypeBySlug[c]
	return ok
}

func (c CommodityType) Validate() error {
	if !c.IsValid() {
		return validation.NewError("invalid_type", "invalid type")
	}
	return nil
}

// MaxCommodityBrandRunes and MaxCommodityModelRunes cap an item's brand and
// model. They count runes, as the frontend counts code points.
const (
	MaxCommodityBrandRunes = 100
	MaxCommodityModelRunes = 100
)

var (
	_ validation.Validatable            = (*Commodity)(nil)
	_ validation.ValidatableWithContext = (*Commodity)(nil)
	_ TenantGroupAwareIDable            = (*Commodity)(nil)
	_ json.Marshaler                    = (*Commodity)(nil)
	_ json.Unmarshaler                  = (*Commodity)(nil)
)

// Enable RLS for multi-tenant isolation
//ptah:schema:rls:enable table="commodities" comment="Enable RLS for multi-tenant commodity isolation"
//ptah:schema:rls:policy name="commodity_isolation" table="commodities" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures commodities can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="commodity_background_worker_access" table="commodities" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all commodities for processing"

// Both-or-neither invariant on the acquisition pair (#1550 / #202) is
// enforced at the application layer: migrationops.SetAcquisition is
// the only writer, and it always writes the pair atomically inside
// TX2; CommodityRegistry.Create drops user-supplied values, Update
// preserves them. A schema-level CHECK constraint would be nice as
// defence in depth, but ptah's walker.go does NOT bubble per-file
// `Database.Constraints` from ParseFS results, so any
// `ptah:schema:constraint` annotation drifts vs the live DB on
// every run. Re-add when the upstream walker is fixed.
//
//ptah:schema:table name="commodities"
type Commodity struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID
	//ptah:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	//ptah:schema:field name="short_name" type="TEXT"
	ShortName string `json:"short_name" db:"short_name"`
	// Brand is the maker of the item ("Bosch"), as plain text. The brand
	// input suggests the brands the group already uses (see
	// CommodityRegistry.ListBrands), and the item search matches it.
	//ptah:schema:field name="brand" type="TEXT" not_null="true" default=""
	Brand string `json:"brand" db:"brand"`
	// Model is the maker's model name or number ("SMV6ZCX42E"), as plain
	// text.
	//ptah:schema:field name="model" type="TEXT" not_null="true" default=""
	Model string `json:"model" db:"model"`
	// Type is the slug of one of the group's item types (see ItemType). An
	// item keeps the slug, not the type's id: a slug never changes, so the
	// item query's type filter, saved views and backups that name it stay
	// valid when the type is renamed.
	//ptah:schema:field name="type" type="TEXT" not_null="true"
	Type CommodityType `json:"type" db:"type" swaggertype:"string" example:"electronics"`
	// AreaID is the area the commodity is filed under. Nullable (issue
	// #1986): a commodity may be created with no area and later
	// un-assigned. Mirrors CoverFileID's nullable-FK pointer style. No
	// on_delete clause — area deletion is handled at the service layer
	// (EntityService.DeleteAreaRecursive cascades to the area's
	// commodities), unchanged by this issue.
	//ptah:schema:field name="area_id" type="TEXT" foreign="areas(id)" foreign_key_name="fk_commodity_area"
	AreaID *string `json:"area_id,omitempty" db:"area_id"`
	//ptah:schema:field name="count" type="INTEGER" not_null="true" default="1"
	Count int `json:"count" db:"count"`
	// SellerID is the seller the item was bought from (#2804), or nil.
	// Deleting the seller leaves the item without one. The seller must be in
	// the item's group: the postgres registry looks it up under the group's
	// row-level security before every write, since the foreign key alone
	// would accept a seller of any group.
	//ptah:schema:field name="seller_id" type="TEXT" foreign="sellers(id)" foreign_key_name="fk_commodity_seller" on_delete="SET NULL"
	SellerID *string `json:"seller_id,omitempty" db:"seller_id"`
	// OriginalPrice is the purchase price in OriginalPriceCurrency, null when
	// unknown. Each of the three prices can be unknown on its own, and
	// unknown is not zero: a gift's price is unknown, a free item's is 0. A
	// reviewed item may leave any of them unknown.
	//ptah:schema:field name="original_price" type="DECIMAL(15,2)"
	OriginalPrice *decimal.Decimal `json:"original_price" db:"original_price" extensions:"x-nullable=true"`
	//ptah:schema:field name="original_price_currency" type="TEXT"
	OriginalPriceCurrency Currency `json:"original_price_currency" db:"original_price_currency"`
	// ConvertedOriginalPrice is the purchase price in the group currency, null
	// when unknown. It applies only when OriginalPriceCurrency is a foreign
	// currency; for an item bought in the group currency it is always null.
	//ptah:schema:field name="converted_original_price" type="DECIMAL(15,2)"
	ConvertedOriginalPrice *decimal.Decimal `json:"converted_original_price" db:"converted_original_price" extensions:"x-nullable=true"`
	// CurrentPrice is the current value in the group currency, null when
	// unknown. When known, 0 included, it is the item's value in every total;
	// otherwise the purchase price in the group currency stands in.
	//ptah:schema:field name="current_price" type="DECIMAL(15,2)"
	CurrentPrice *decimal.Decimal `json:"current_price" db:"current_price" extensions:"x-nullable=true"`
	//ptah:schema:field name="serial_number" type="TEXT"
	SerialNumber string `json:"serial_number" db:"serial_number"`
	//ptah:schema:field name="extra_serial_numbers" type="JSONB"
	ExtraSerialNumbers ValuerSlice[string] `json:"extra_serial_numbers" db:"extra_serial_numbers"`
	//ptah:schema:field name="part_numbers" type="JSONB"
	PartNumbers ValuerSlice[string] `json:"part_numbers" db:"part_numbers"`
	//ptah:schema:field name="tags" type="JSONB"
	Tags ValuerSlice[string] `json:"tags" db:"tags"`
	//ptah:schema:field name="status" type="TEXT" not_null="true"
	Status CommodityStatus `json:"status" db:"status"`
	// PurchaseDate is nil when the date is unknown.
	//ptah:schema:field name="purchase_date" type="TEXT"
	PurchaseDate PDate `json:"purchase_date" db:"purchase_date" extensions:"x-nullable=true"`
	//ptah:schema:field name="registered_date" type="TEXT"
	RegisteredDate PDate `json:"registered_date" db:"registered_date"`
	//ptah:schema:field name="last_modified_date" type="TEXT"
	LastModifiedDate PDate `json:"last_modified_date" db:"last_modified_date"`
	//ptah:schema:field name="urls" type="JSONB"
	URLs ValuerSlice[*URL] `json:"urls" swaggertype:"string" db:"urls"`
	//ptah:schema:field name="comments" type="TEXT"
	Comments string `json:"comments" db:"comments"`
	//ptah:schema:field name="draft" type="BOOLEAN" not_null="true" default="false"
	Draft bool `json:"draft" db:"draft"`
	// CoverFileID is the user-picked cover photo for the commodity (issue
	// #1451 option B). Nullable: when unset, the cover-resolver falls back
	// to the earliest `category=images` file (option A — first photo).
	// ON DELETE SET NULL so deleting the photo silently drops the
	// override; the resolver's first-photo path takes over.
	//ptah:schema:field name="cover_file_id" type="TEXT" foreign="files(id)" foreign_key_name="fk_commodity_cover_file" on_delete="SET NULL"
	CoverFileID *string `json:"cover_file_id,omitempty" db:"cover_file_id"`
	// WarrantyExpiresAt is the date the manufacturer/seller warranty for this
	// commodity ends. Nil means "no warranty tracked" (status=none). Status —
	// active / expiring / expired — is computed from this date and the server
	// clock, never stored, so a row "expires" without any write happening.
	//ptah:schema:field name="warranty_expires_at" type="TEXT"
	WarrantyExpiresAt PDate `json:"warranty_expires_at" db:"warranty_expires_at"`
	//ptah:schema:field name="warranty_notes" type="TEXT"
	WarrantyNotes string `json:"warranty_notes" db:"warranty_notes"`

	// AcquisitionPrice is the per-row "as purchased" amount, frozen the
	// first time a currency migration overwrites OriginalPrice for this
	// commodity (Case A in issue #202 §2). NULL until that point — a
	// fresh commodity does not need it because the live OriginalPrice
	// already is the purchase value. Server-managed and write-once: the
	// API silently drops any payload values, and the migration worker
	// only writes when both columns are still NULL.
	//ptah:schema:field name="acquisition_price" type="DECIMAL(15,2)"
	AcquisitionPrice *decimal.Decimal `json:"acquisition_price,omitempty" db:"acquisition_price" userinput:"false" readonly:"true"`
	// AcquisitionCurrency is the original currency of AcquisitionPrice.
	// Always either both NULL or both set (DB CHECK constraint enforces).
	//ptah:schema:field name="acquisition_currency" type="TEXT"
	AcquisitionCurrency *Currency `json:"acquisition_currency,omitempty" db:"acquisition_currency" userinput:"false" readonly:"true"`

	// StatusDate is the day the user reported the commodity's transition
	// out of `in_use`. Captured by the FE `StatusTransitionDialog` (issue
	// #1611) and surfaced on the terminal-status info card. PDate is a
	// day-precision string — the mock uses `<input type="date">` so we
	// don't need sub-day granularity. NULL while `status = in_use` or
	// for terminal rows that pre-date this column.
	//ptah:schema:field name="status_date" type="TEXT"
	StatusDate PDate `json:"status_date" db:"status_date"`
	// StatusNote is the free-form note recorded alongside a status
	// transition (e.g. "Sold to Bob" / "Last seen at the airport"). NULL
	// for `in_use` rows and pre-existing terminal rows. Bounded by the
	// same 1024-char ceiling we use for Comments to keep the BE/UI sane.
	//ptah:schema:field name="status_note" type="TEXT"
	StatusNote string `json:"status_note" db:"status_note"`
	// SalePrice is the realised proceeds for a `sold` transition. NULL
	// for any other status. Stored in the commodity's original purchase
	// currency — sale-side currency reporting is out of scope for #1611.
	//ptah:schema:field name="sale_price" type="DECIMAL(15,2)"
	SalePrice *decimal.Decimal `json:"sale_price,omitempty" db:"sale_price"`

	// AssetNumber is the short sequential number of the item inside its
	// group, displayed as an asset ID such as "A-0042" (see FormatAssetID).
	// The registry assigns it on Create as the group's highest number plus
	// one, under a per-group lock, so concurrent creates never share a
	// number. It is server-managed: Create ignores a caller-supplied value
	// (except for the trusted restore seam, registry.WithRestoreAssetNumber)
	// and Update keeps the stored one. NULL only for rows that predate the
	// column until the runtime backfill numbers them.
	//ptah:schema:field name="asset_number" type="INTEGER"
	AssetNumber *int `json:"asset_number,omitempty" db:"asset_number" userinput:"false" readonly:"true"`

	// SearchText is a database-generated column holding the text fields
	// the item search reads (registry.CommoditySearchTextFields), in that
	// order, joined by the unit separator U+001F. The brand and the model
	// are matched on their own columns instead: adding them here would
	// change the function's parameters, which the column depends on, and
	// rewriting a generated column's expression in place needs
	// PostgreSQL 17. The free-text search
	// matches each term against it with ILIKE, which the trigram index
	// commodities_search_text_trgm_idx serves. A search term never contains
	// a control character, so a match cannot span two fields. PostgreSQL
	// computes the value on every write; the application never sets it,
	// which `dbwrite:"false"` tells the INSERT/UPDATE builder.
	//ptah:schema:field name="search_text" type="TEXT" generated="commodity_search_text(name, short_name, serial_number, extra_serial_numbers, part_numbers, tags, comments, warranty_notes, status_note)" stored="true"
	SearchText string `json:"-" db:"search_text" dbwrite:"false" userinput:"false" readonly:"true"`

	// LegacyZeroPrices marks a row whose prices were stored when a price
	// could not be unknown: a zero current or converted price in such a row
	// means "not set". The runtime backfill
	// (CommodityRegistry.BackfillLegacyPrices) turns those zeros into NULL and
	// clears the flag. The column defaults to true so that adding it marks
	// every row that already exists; the registries store false on every
	// write, and the flag never leaves the server.
	//ptah:schema:field name="legacy_zero_prices" type="BOOLEAN" not_null="true" default="true"
	LegacyZeroPrices bool `json:"-" db:"legacy_zero_prices"`
	// DeletedAt is when the item was moved to Trash, nil while it is live. A
	// trashed item keeps its id, asset number, history, loans, service
	// records, maintenance schedules and attachments, and is left out of every
	// ordinary read of the registries until it is restored or purged (see
	// registry.CommodityRegistry). Server-managed: only the registry's trash
	// operations write it; Create and Update ignore a caller's value.
	//ptah:schema:field name="deleted_at" type="TIMESTAMP"
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at" userinput:"false" readonly:"true"`
	// DeletedBy is the id of the user who moved the item to Trash.
	//ptah:schema:field name="deleted_by" type="TEXT"
	DeletedBy *string `json:"deleted_by,omitempty" db:"deleted_by" userinput:"false" readonly:"true"`
	// DeletedFromAreaID is the area the item was filed under when it was moved
	// to Trash. Deleting that area clears AreaID of the trashed item (the
	// foreign key allows nothing else) but keeps this column, so a restore can
	// tell "was never in an area" from "its area is gone" and say so.
	//ptah:schema:field name="deleted_from_area_id" type="TEXT"
	DeletedFromAreaID *string `json:"-" db:"deleted_from_area_id" userinput:"false" readonly:"true"`
}

// PurchasePriceInGroupCurrency returns the purchase price in the group
// currency and whether it is known. For an item bought in the group currency
// that is the original price alone: a converted price does not apply to it,
// so an unknown original price stays unknown even when a converted one is
// stored. For an item bought in another currency it is the converted price.
func (a *Commodity) PurchasePriceInGroupCurrency(groupCurrency Currency) (decimal.Decimal, bool) {
	price := a.ConvertedOriginalPrice
	if a.OriginalPriceCurrency == groupCurrency {
		price = a.OriginalPrice
	}
	if price == nil {
		return decimal.Zero, false
	}
	return *price, true
}

// NormalizePrices drops the converted price of an item bought in the group
// currency. There is nothing to convert, and clients written before prices
// could be unknown send a zero there for "not entered"; storing it would
// leave a zero that means nothing. The registries call it on every write
// whose context carries the group currency.
func (a *Commodity) NormalizePrices(groupCurrency Currency) {
	if groupCurrency != "" && a.OriginalPriceCurrency == groupCurrency {
		a.ConvertedOriginalPrice = nil
	}
}

// UnsetEchoedLegacyZeros keeps a legacy "not entered" zero unknown across a
// full-row write. While the stored row is still flagged LegacyZeroPrices
// (the backfill has not reached it), a zero current or converted price in it
// means "not set", and every reader saw it as 0. A write that sends that same
// zero back is echoing what it read, so the price is stored as unknown, as
// the backfill would store it; without this the write would clear the flag
// and turn the legacy zero into a known one for good. Any other value,
// unknown included, is the caller's own and is kept.
func (a *Commodity) UnsetEchoedLegacyZeros(stored *Commodity) {
	if stored == nil || !stored.LegacyZeroPrices {
		return
	}
	if isZeroPrice(stored.CurrentPrice) && isZeroPrice(a.CurrentPrice) {
		a.CurrentPrice = nil
	}
	if isZeroPrice(stored.ConvertedOriginalPrice) && isZeroPrice(a.ConvertedOriginalPrice) {
		a.ConvertedOriginalPrice = nil
	}
}

func isZeroPrice(p *decimal.Decimal) bool {
	return p != nil && p.IsZero()
}

// Value returns the value of the commodity in the group currency and whether
// it is known. A known current price wins; otherwise the purchase price in the
// group currency counts (see PurchasePriceInGroupCurrency). A known zero is a
// value. A commodity with neither has no usable value, and totals report it
// separately instead of adding it as zero.
func (a *Commodity) Value(groupCurrency Currency) (decimal.Decimal, bool) {
	if a.CurrentPrice != nil {
		return *a.CurrentPrice, true
	}
	return a.PurchasePriceInGroupCurrency(groupCurrency)
}

// SamePrice reports whether two prices are equal. An unknown (nil) price
// equals only another unknown price, and the comparison ignores scale, so
// 17500.00 equals 17500.
func SamePrice(a, b *decimal.Decimal) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// CommoditySearchFunctions declares the function behind the generated
// commodities.search_text column. It joins the text columns and the strings
// of the three JSONB arrays (extra serial numbers, part numbers, tag slugs)
// with U+001F, in the order of registry.CommoditySearchTextFields. It is
// IMMUTABLE, which PostgreSQL requires of a generated column, and it
// qualifies every catalog name with pg_catalog so the result does not depend
// on the search_path of the role that writes the row. A JSONB value that is
// not an array contributes an empty string, so a NULL cannot fail an INSERT.
//
// The concatenation lives in a function rather than in the column's
// expression because PostgreSQL stores an expression in a normalized form
// (every || parenthesized), and the schema drift check compares that form
// with the annotation; a plain call reads back unchanged.
//
// The annotation sits on the type's doc comment: Ptah collects
// `ptah:schema:function` from the declaration a comment is attached to.
//
//ptah:schema:function name="commodity_search_text" params="name TEXT, short_name TEXT, serial_number TEXT, extra_serial_numbers JSONB, part_numbers JSONB, tags JSONB, comments TEXT, warranty_notes TEXT, status_note TEXT" returns="TEXT" language="sql" volatility="IMMUTABLE" body="SELECT COALESCE(name, '') || pg_catalog.chr(31) || COALESCE(short_name, '') || pg_catalog.chr(31) || COALESCE(serial_number, '') || pg_catalog.chr(31) || COALESCE((SELECT pg_catalog.string_agg(e, pg_catalog.chr(31)) FROM pg_catalog.jsonb_array_elements_text(CASE WHEN pg_catalog.jsonb_typeof(extra_serial_numbers) = 'array' THEN extra_serial_numbers END) AS e), '') || pg_catalog.chr(31) || COALESCE((SELECT pg_catalog.string_agg(e, pg_catalog.chr(31)) FROM pg_catalog.jsonb_array_elements_text(CASE WHEN pg_catalog.jsonb_typeof(part_numbers) = 'array' THEN part_numbers END) AS e), '') || pg_catalog.chr(31) || COALESCE((SELECT pg_catalog.string_agg(e, pg_catalog.chr(31)) FROM pg_catalog.jsonb_array_elements_text(CASE WHEN pg_catalog.jsonb_typeof(tags) = 'array' THEN tags END) AS e), '') || pg_catalog.chr(31) || COALESCE(comments, '') || pg_catalog.chr(31) || COALESCE(warranty_notes, '') || pg_catalog.chr(31) || COALESCE(status_note, '')" comment="Builds commodities.search_text, the text the item search matches"
type CommoditySearchFunctions struct {
	_ int
}

// IsTrashed reports whether the item is in Trash.
func (a *Commodity) IsTrashed() bool {
	return a != nil && a.DeletedAt != nil
}

// ClearTrashState drops the trash columns. The registries call it on Create so
// a caller (an API payload, a backup archive) cannot insert an item that is
// already in Trash.
func (a *Commodity) ClearTrashState() {
	a.DeletedAt = nil
	a.DeletedBy = nil
	a.DeletedFromAreaID = nil
}

// assetIDPrefix and assetIDDigits define the display form of an asset
// number: "A-" followed by the number zero-padded to four digits. Numbers
// past 9999 keep all their digits ("A-12345").
const (
	assetIDPrefix = "A-"
	assetIDDigits = 4
)

// FormatAssetID renders an asset number in its display form, "A-0042".
// Non-positive numbers render as an empty string: they are never assigned.
func FormatAssetID(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%s%0*d", assetIDPrefix, assetIDDigits, n)
}

// ParseAssetID reads an asset number back from what a user types into a
// search box. It accepts the display form and its loose variants, all
// case-insensitive: "A-0042", "a-42", "A42", "A 42", "#42" and a bare "42".
// The second result is false for anything else, including zero.
func ParseAssetID(s string) (int, bool) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "#"):
		s = s[1:]
	case len(s) > 0 && (s[0] == 'a' || s[0] == 'A'):
		s = strings.TrimLeft(s[1:], "- ")
	}
	if s == "" || len(s) > 9 {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// WarrantyStatus is the computed warranty state of a commodity. It is
// derived from Commodity.WarrantyExpiresAt and the server clock — never
// stored, never returned by the registry layer. The list endpoint accepts
// it as a filter and the FE renders the matching pill.
type WarrantyStatus string

const (
	// WarrantyStatusNone — no expiry date set on the commodity.
	WarrantyStatusNone WarrantyStatus = "none"
	// WarrantyStatusActive — expiry date is set and more than
	// WarrantyExpiringWindowDays in the future.
	WarrantyStatusActive WarrantyStatus = "active"
	// WarrantyStatusExpiring — expiry date is within
	// WarrantyExpiringWindowDays of now (inclusive at both ends).
	WarrantyStatusExpiring WarrantyStatus = "expiring"
	// WarrantyStatusExpired — expiry date has already passed.
	WarrantyStatusExpired WarrantyStatus = "expired"
)

// WarrantyExpiringWindowDays is the threshold below which an active
// warranty flips to "expiring". Matches the earliest reminder cadence (60
// days, see WarrantyReminderThresholds) so that an item that earns a
// reminder is also surfaced in the FE's "Expiring soon" tab on the same
// day.
const WarrantyExpiringWindowDays = 60

// IsValid reports whether s is one of the documented warranty statuses.
// Empty string is invalid — callers should choose explicitly between
// "none" (unfilterable) and a real status.
func (s WarrantyStatus) IsValid() bool {
	switch s {
	case WarrantyStatusNone, WarrantyStatusActive, WarrantyStatusExpiring, WarrantyStatusExpired:
		return true
	}
	return false
}

// ComputeWarrantyStatus returns the derived status for the given expiry
// date relative to now. Nil expiry → none. The "expiring" window is
// closed on both ends (today and exactly 60 days from now both count).
//
// `now` is normalised to UTC before deriving today's date — passing a
// non-UTC `time.Now()` (e.g., server local time) without the
// normalisation would compute the wrong UTC day near midnight and
// misclassify the row by ±1 day. The string-based SQL filter in
// postgres also anchors on UTC, so this keeps the two paths in sync.
func ComputeWarrantyStatus(expires PDate, now time.Time) WarrantyStatus {
	if expires == nil || string(*expires) == "" {
		return WarrantyStatusNone
	}
	exp := expires.ToTime()
	if exp.IsZero() {
		return WarrantyStatusNone
	}
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	if exp.Before(today) {
		return WarrantyStatusExpired
	}
	cutoff := today.AddDate(0, 0, WarrantyExpiringWindowDays)
	if !exp.After(cutoff) {
		return WarrantyStatusExpiring
	}
	return WarrantyStatusActive
}

// PostgreSQL-specific indexes for commodities
type CommodityIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//ptah:schema:index name="idx_commodities_uuid" fields="uuid" unique="true" table="commodities"
	_ int

	// Index for tenant-based queries
	//ptah:schema:index name="idx_commodities_tenant_id" fields="tenant_id" table="commodities"
	_ int

	// An asset number is unique inside its group. The index also serves the
	// MAX(asset_number) lookup that assigns the next number and the search
	// by asset ID. NULLs (rows not yet backfilled) do not collide.
	//ptah:schema:index name="idx_commodities_group_asset_number" fields="group_id,asset_number" unique="true" table="commodities"
	_ int

	// Composite index for tenant + area queries
	//ptah:schema:index name="idx_commodities_tenant_area" fields="tenant_id,area_id" table="commodities"
	_ int

	// The seller filter of the items list and the item counts per seller.
	//ptah:schema:index name="idx_commodities_seller" fields="seller_id" table="commodities"
	_ int

	// Composite index for tenant + status queries
	//ptah:schema:index name="idx_commodities_tenant_status" fields="tenant_id,status" table="commodities"
	_ int

	// The type filter of the items list, the item counts per type, and the
	// in-use check and reassignment when an item type is deleted.
	//ptah:schema:index name="idx_commodities_group_type" fields="group_id,type" table="commodities"
	_ int

	// Composite index for tenant+group RLS-filtered queries (e.g. list-by-group)
	//ptah:schema:index name="idx_commodities_tenant_group" fields="tenant_id,group_id" table="commodities"
	_ int

	// GIN index for JSONB tags field
	//ptah:schema:index name="commodities_tags_gin_idx" fields="tags" type="GIN" table="commodities"
	_ int

	// GIN index for JSONB extra_serial_numbers field
	//ptah:schema:index name="commodities_extra_serial_numbers_gin_idx" fields="extra_serial_numbers" type="GIN" table="commodities"
	_ int

	// GIN index for JSONB part_numbers field
	//ptah:schema:index name="commodities_part_numbers_gin_idx" fields="part_numbers" type="GIN" table="commodities"
	_ int

	// GIN index for JSONB urls field
	//ptah:schema:index name="commodities_urls_gin_idx" fields="urls" type="GIN" table="commodities"
	_ int

	// Partial index for active commodities (non-draft)
	//ptah:schema:index name="commodities_active_idx" fields="status,area_id" condition="draft = false" table="commodities"
	_ int

	// Partial index for draft commodities
	//ptah:schema:index name="commodities_draft_idx" fields="last_modified_date" condition="draft = true" table="commodities"
	_ int

	// Trigram similarity index for commodity name search
	//ptah:schema:index name="commodities_name_trgm_idx" fields="name" type="GIN" ops="gin_trgm_ops" table="commodities"
	_ int

	// Trigram similarity index for short name search
	//ptah:schema:index name="commodities_short_name_trgm_idx" fields="short_name" type="GIN" ops="gin_trgm_ops" table="commodities"
	_ int

	// Trigram index for the item search: every term is an ILIKE substring
	// match against the generated search_text column.
	//ptah:schema:index name="commodities_search_text_trgm_idx" fields="search_text" type="GIN" ops="gin_trgm_ops" table="commodities"
	_ int

	// Trigram indexes for the brand and model, which the item search matches
	// on their own columns (registry.CommodityColumnSearchFields) next to
	// search_text, and the brand filter compares case-insensitively.
	//ptah:schema:index name="commodities_brand_trgm_idx" fields="brand" type="GIN" ops="gin_trgm_ops" table="commodities"
	_ int
	//ptah:schema:index name="commodities_model_trgm_idx" fields="model" type="GIN" ops="gin_trgm_ops" table="commodities"
	_ int

	// Partial index for warranty filtering — only commodities that have a
	// warranty date set are interesting for the worker scan and the
	// "expiring soon" filter. Skips the bulk of rows (no warranty).
	//ptah:schema:index name="commodities_warranty_expires_at_idx" fields="warranty_expires_at" condition="warranty_expires_at IS NOT NULL" table="commodities"
	_ int

	// Partial index for Trash: the group's trash list and the purge worker's
	// scan for expired items read only trashed rows, ordered by deletion time.
	//ptah:schema:index name="commodities_deleted_at_idx" fields="deleted_at" condition="deleted_at IS NOT NULL" table="commodities"
	_ int
}

func (*Commodity) Validate() error {
	return ErrMustUseValidateWithContext
}

// NormalizeSellerID collapses an explicit empty seller to nil, as
// NormalizeAreaID does for the area.
func (a *Commodity) NormalizeSellerID() {
	if a.SellerID != nil && *a.SellerID == "" {
		a.SellerID = nil
	}
}

// NormalizeAreaID collapses an explicit empty area to nil so "" and a nil
// pointer both mean "unassigned" (issue #1986). Without it, a payload with
// `"area_id": ""` would persist as an empty string that the `area_id IS NULL`
// filter misses and that matches no real area. Registries call this before
// persisting so a stored area_id is always either NULL or a real area id.
func (a *Commodity) NormalizeAreaID() {
	if a.AreaID != nil && *a.AreaID == "" {
		a.AreaID = nil
	}
}

func (a *Commodity) ValidateWithContext(ctx context.Context) error {
	groupCurrency, err := validationctx.GroupCurrencyFromContext(ctx)
	if errors.Is(err, validationctx.ErrGroupCurrencyNotSet) {
		return validation.NewError("group_currency_not_set", "group currency not set")
	}
	if err != nil {
		return err
	}

	fields := make([]*validation.FieldRules, 0)

	// Create a validation rule for price consistency
	priceRule := rules.NewPriceRule(
		string(groupCurrency),
		string(a.OriginalPriceCurrency),
		a.OriginalPrice,
		a.ConvertedOriginalPrice,
		a.CurrentPrice,
	)

	whenNotDraft := rules.WhenTrue(!a.Draft) // Rule to apply rules when not draft

	// A known amount must not be negative. An unknown (nil) amount passes:
	// a reviewed item may leave its purchase date and every price unknown.
	nonNegative := func(price *decimal.Decimal) validation.Rule {
		return validation.By(func(any) error {
			if price == nil {
				return nil
			}
			v, _ := price.Float64()
			return validation.Min(0.00).Validate(v)
		})
	}

	fields = append(fields,
		validation.Field(&a.Name, rules.NotEmpty, validation.Length(1, 255)),
		validation.Field(&a.ShortName, rules.NotEmpty, validation.Length(1, 40)),
		validation.Field(&a.Brand, validation.RuneLength(0, MaxCommodityBrandRunes)),
		validation.Field(&a.Model, validation.RuneLength(0, MaxCommodityModelRunes)),
		validation.Field(&a.Type, rules.NotEmpty),
		validation.Field(&a.Status, rules.NotEmpty),
		// The purchase date is optional on every item, draft or reviewed.
		validation.Field(&a.PurchaseDate, DateFormat),
		validation.Field(&a.Count, validation.Required, validation.Min(1)),
		// #1554: Count > 1 is a "bundle of identical units" semantics, not
		// a single tracked instance. Warranty fields describe a single
		// instance's manufacturer cover, so a bundle row carrying a
		// warranty is nonsensical. Reject at write-time; the FE form
		// disables the inputs and surfaces a banner. Loan / service
		// records sit on their own tables and are gated in the
		// per-table service layer (services.EnsureCommodityTrackable).
		validation.Field(&a.WarrantyExpiresAt, validation.By(func(any) error {
			if a.Count > 1 && a.WarrantyExpiresAt != nil && string(*a.WarrantyExpiresAt) != "" {
				return validation.NewError("quantity_forbids_warranty",
					"warranty cannot be tracked on commodities with quantity > 1")
			}
			return nil
		})),
		validation.Field(&a.WarrantyNotes, validation.By(func(any) error {
			if a.Count > 1 && a.WarrantyNotes != "" {
				return validation.NewError("quantity_forbids_warranty_notes",
					"warranty notes cannot be set on commodities with quantity > 1")
			}
			return nil
		})),
		validation.Field(&a.URLs),
		// The cross-field PriceRule is the single source of truth for which
		// prices a known amount requires; no price is required on its own.
		validation.Field(&a.OriginalPrice, whenNotDraft.WithRules(priceRule, nonNegative(a.OriginalPrice))),
		// The currency is required on a reviewed item even when its
		// purchase price is unknown: it is the currency the price is
		// entered in once it becomes known.
		validation.Field(&a.OriginalPriceCurrency, whenNotDraft.WithRules(validation.Required)),
		validation.Field(&a.ConvertedOriginalPrice, whenNotDraft.WithRules(nonNegative(a.ConvertedOriginalPrice))),
		validation.Field(&a.CurrentPrice, whenNotDraft.WithRules(nonNegative(a.CurrentPrice))),
		// #1611: terminal-status metadata. The cross-field "required
		// when the transition leaves in_use" check lives in the
		// updateCommodity handler (it needs the previous status,
		// which the model layer doesn't carry); these rules cover the
		// per-row invariants that hold on every write.
		validation.Field(&a.StatusNote, validation.Length(0, 1024)),
		validation.Field(&a.SalePrice, validation.By(func(any) error {
			if a.SalePrice == nil {
				return nil
			}
			if a.Status != CommodityStatusSold {
				return validation.NewError("status_forbids_sale_price",
					"sale price can only be recorded when status is sold")
			}
			v, _ := a.SalePrice.Float64()
			return validation.Min(0.00).Validate(v)
		})),
		validation.Field(&a.StatusDate, validation.By(func(any) error {
			if a.StatusDate == nil || string(*a.StatusDate) == "" {
				return nil
			}
			if a.Status == CommodityStatusInUse {
				return validation.NewError("status_forbids_status_date",
					"status date cannot be set when status is in_use")
			}
			return nil
		})),
	)

	return validation.ValidateStructWithContext(ctx, a, fields...)
}

func (a *Commodity) MarshalJSON() ([]byte, error) {
	type Alias Commodity
	tmp := *a
	return json.Marshal(Alias(tmp))
}

func (a *Commodity) UnmarshalJSON(data []byte) error {
	type Alias Commodity
	tmp := &Alias{}
	err := json.Unmarshal(data, tmp)
	if err != nil {
		return err
	}

	*a = Commodity(*tmp)
	return nil
}
