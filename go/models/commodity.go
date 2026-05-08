package models

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models/rules"
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

type CommodityType string

// Commodity types. Adding a new type? Don't forget to update IsValid() method.
const (
	CommodityTypeWhiteGoods  CommodityType = "white_goods"
	CommodityTypeElectronics CommodityType = "electronics"
	CommodityTypeEquipment   CommodityType = "equipment"
	CommodityTypeFurniture   CommodityType = "furniture"
	CommodityTypeClothes     CommodityType = "clothes"
	CommodityTypeOther       CommodityType = "other"
)

func (c CommodityType) IsValid() bool {
	switch c {
	case CommodityTypeWhiteGoods,
		CommodityTypeElectronics,
		CommodityTypeEquipment,
		CommodityTypeFurniture,
		CommodityTypeClothes,
		CommodityTypeOther:
		return true
	}
	return false
}

func (c CommodityType) Validate() error {
	if !c.IsValid() {
		return validation.NewError("invalid_type", "invalid type")
	}
	return nil
}

var (
	_ validation.Validatable            = (*Commodity)(nil)
	_ validation.ValidatableWithContext = (*Commodity)(nil)
	_ TenantGroupAwareIDable            = (*Commodity)(nil)
	_ json.Marshaler                    = (*Commodity)(nil)
	_ json.Unmarshaler                  = (*Commodity)(nil)
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="commodities" comment="Enable RLS for multi-tenant commodity isolation"
//migrator:schema:rls:policy name="commodity_isolation" table="commodities" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures commodities can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="commodity_background_worker_access" table="commodities" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all commodities for processing"

// Both-or-neither invariant on the acquisition pair (#1550 / #202) is
// enforced at the application layer: migrationops.SetAcquisition is
// the only writer, and it always writes the pair atomically inside
// TX2; CommodityRegistry.Create drops user-supplied values, Update
// preserves them. A schema-level CHECK constraint would be nice as
// defence in depth, but ptah's walker.go does NOT bubble per-file
// `Database.Constraints` from ParseFS results, so any
// `migrator:schema:constraint` annotation drifts vs the live DB on
// every run. Re-add when the upstream walker is fixed.
//
//migrator:schema:table name="commodities"
type Commodity struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	//migrator:schema:field name="short_name" type="TEXT"
	ShortName string `json:"short_name" db:"short_name"`
	//migrator:schema:field name="type" type="TEXT" not_null="true"
	Type CommodityType `json:"type" db:"type"`
	//migrator:schema:field name="area_id" type="TEXT" not_null="true" foreign="areas(id)" foreign_key_name="fk_commodity_area"
	AreaID string `json:"area_id" db:"area_id"`
	//migrator:schema:field name="count" type="INTEGER" not_null="true" default="1"
	Count int `json:"count" db:"count"`
	//migrator:schema:field name="original_price" type="DECIMAL(15,2)"
	OriginalPrice decimal.Decimal `json:"original_price" db:"original_price"`
	//migrator:schema:field name="original_price_currency" type="TEXT"
	OriginalPriceCurrency Currency `json:"original_price_currency" db:"original_price_currency"`
	//migrator:schema:field name="converted_original_price" type="DECIMAL(15,2)"
	ConvertedOriginalPrice decimal.Decimal `json:"converted_original_price" db:"converted_original_price"`
	//migrator:schema:field name="current_price" type="DECIMAL(15,2)"
	CurrentPrice decimal.Decimal `json:"current_price" db:"current_price"`
	//migrator:schema:field name="serial_number" type="TEXT"
	SerialNumber string `json:"serial_number" db:"serial_number"`
	//migrator:schema:field name="extra_serial_numbers" type="JSONB"
	ExtraSerialNumbers ValuerSlice[string] `json:"extra_serial_numbers" db:"extra_serial_numbers"`
	//migrator:schema:field name="part_numbers" type="JSONB"
	PartNumbers ValuerSlice[string] `json:"part_numbers" db:"part_numbers"`
	//migrator:schema:field name="tags" type="JSONB"
	Tags ValuerSlice[string] `json:"tags" db:"tags"`
	//migrator:schema:field name="status" type="TEXT" not_null="true"
	Status CommodityStatus `json:"status" db:"status"`
	//migrator:schema:field name="purchase_date" type="TEXT"
	PurchaseDate PDate `json:"purchase_date" db:"purchase_date"`
	//migrator:schema:field name="registered_date" type="TEXT"
	RegisteredDate PDate `json:"registered_date" db:"registered_date"`
	//migrator:schema:field name="last_modified_date" type="TEXT"
	LastModifiedDate PDate `json:"last_modified_date" db:"last_modified_date"`
	//migrator:schema:field name="urls" type="JSONB"
	URLs ValuerSlice[*URL] `json:"urls" swaggertype:"string" db:"urls"`
	//migrator:schema:field name="comments" type="TEXT"
	Comments string `json:"comments" db:"comments"`
	//migrator:schema:field name="draft" type="BOOLEAN" not_null="true" default="false"
	Draft bool `json:"draft" db:"draft"`
	// CoverFileID is the user-picked cover photo for the commodity (issue
	// #1451 option B). Nullable: when unset, the cover-resolver falls back
	// to the earliest `category=photos` file (option A — first photo).
	// ON DELETE SET NULL so deleting the photo silently drops the
	// override; the resolver's first-photo path takes over.
	//migrator:schema:field name="cover_file_id" type="TEXT" foreign="files(id)" foreign_key_name="fk_commodity_cover_file" on_delete="SET NULL"
	CoverFileID *string `json:"cover_file_id,omitempty" db:"cover_file_id"`
	// WarrantyExpiresAt is the date the manufacturer/seller warranty for this
	// commodity ends. Nil means "no warranty tracked" (status=none). Status —
	// active / expiring / expired — is computed from this date and the server
	// clock, never stored, so a row "expires" without any write happening.
	//migrator:schema:field name="warranty_expires_at" type="TEXT"
	WarrantyExpiresAt PDate `json:"warranty_expires_at" db:"warranty_expires_at"`
	//migrator:schema:field name="warranty_notes" type="TEXT"
	WarrantyNotes string `json:"warranty_notes" db:"warranty_notes"`

	// AcquisitionPrice is the per-row "as purchased" amount, frozen the
	// first time a currency migration overwrites OriginalPrice for this
	// commodity (Case A in issue #202 §2). NULL until that point — a
	// fresh commodity does not need it because the live OriginalPrice
	// already is the purchase value. Server-managed and write-once: the
	// API silently drops any payload values, and the migration worker
	// only writes when both columns are still NULL.
	//migrator:schema:field name="acquisition_price" type="DECIMAL(15,2)"
	AcquisitionPrice *decimal.Decimal `json:"acquisition_price,omitempty" db:"acquisition_price" userinput:"false" readonly:"true"`
	// AcquisitionCurrency is the original currency of AcquisitionPrice.
	// Always either both NULL or both set (DB CHECK constraint enforces).
	//migrator:schema:field name="acquisition_currency" type="TEXT"
	AcquisitionCurrency *Currency `json:"acquisition_currency,omitempty" db:"acquisition_currency" userinput:"false" readonly:"true"`
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
	//migrator:schema:index name="idx_commodities_uuid" fields="uuid" unique="true" table="commodities"
	_ int

	// Index for tenant-based queries
	//migrator:schema:index name="idx_commodities_tenant_id" fields="tenant_id" table="commodities"
	_ int

	// Composite index for tenant + area queries
	//migrator:schema:index name="idx_commodities_tenant_area" fields="tenant_id,area_id" table="commodities"
	_ int

	// Composite index for tenant + status queries
	//migrator:schema:index name="idx_commodities_tenant_status" fields="tenant_id,status" table="commodities"
	_ int

	// Composite index for tenant+group RLS-filtered queries (e.g. list-by-group)
	//migrator:schema:index name="idx_commodities_tenant_group" fields="tenant_id,group_id" table="commodities"
	_ int

	// GIN index for JSONB tags field
	//migrator:schema:index name="commodities_tags_gin_idx" fields="tags" type="GIN" table="commodities"
	_ int

	// GIN index for JSONB extra_serial_numbers field
	//migrator:schema:index name="commodities_extra_serial_numbers_gin_idx" fields="extra_serial_numbers" type="GIN" table="commodities"
	_ int

	// GIN index for JSONB part_numbers field
	//migrator:schema:index name="commodities_part_numbers_gin_idx" fields="part_numbers" type="GIN" table="commodities"
	_ int

	// GIN index for JSONB urls field
	//migrator:schema:index name="commodities_urls_gin_idx" fields="urls" type="GIN" table="commodities"
	_ int

	// Partial index for active commodities (non-draft)
	//migrator:schema:index name="commodities_active_idx" fields="status,area_id" condition="draft = false" table="commodities"
	_ int

	// Partial index for draft commodities
	//migrator:schema:index name="commodities_draft_idx" fields="last_modified_date" condition="draft = true" table="commodities"
	_ int

	// Trigram similarity index for commodity name search
	//migrator:schema:index name="commodities_name_trgm_idx" fields="name" type="GIN" ops="gin_trgm_ops" table="commodities"
	_ int

	// Trigram similarity index for short name search
	//migrator:schema:index name="commodities_short_name_trgm_idx" fields="short_name" type="GIN" ops="gin_trgm_ops" table="commodities"
	_ int

	// Partial index for warranty filtering — only commodities that have a
	// warranty date set are interesting for the worker scan and the
	// "expiring soon" filter. Skips the bulk of rows (no warranty).
	//migrator:schema:index name="commodities_warranty_expires_at_idx" fields="warranty_expires_at" condition="warranty_expires_at IS NOT NULL" table="commodities"
	_ int
}

func (*Commodity) Validate() error {
	return ErrMustUseValidateWithContext
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

	fields = append(fields,
		validation.Field(&a.Name, rules.NotEmpty, validation.Length(1, 255)),
		validation.Field(&a.ShortName, rules.NotEmpty, validation.Length(1, 20)),
		validation.Field(&a.Type, rules.NotEmpty),
		validation.Field(&a.AreaID, rules.NotEmpty),
		validation.Field(&a.Status, rules.NotEmpty),
		validation.Field(&a.PurchaseDate, whenNotDraft.WithRules(rules.NotEmpty)),
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
				return validation.NewError("quantity_forbids_warranty",
					"warranty notes cannot be set on commodities with quantity > 1")
			}
			return nil
		})),
		validation.Field(&a.URLs),
		validation.Field(&a.OriginalPrice, whenNotDraft.WithRules(priceRule, validation.By(func(any) error {
			v, _ := a.OriginalPrice.Float64()
			return validation.Min(0.00).Validate(v)
		}))),
		validation.Field(&a.OriginalPriceCurrency, whenNotDraft.WithRules(validation.By(func(val any) error {
			if a.Draft {
				return nil
			}

			return validation.Required.Validate(val)
		}))),
		validation.Field(&a.ConvertedOriginalPrice, whenNotDraft.WithRules(validation.Required, validation.By(func(any) error {
			v, _ := a.ConvertedOriginalPrice.Float64()
			return validation.Min(0.00).Validate(v)
		}))),
		validation.Field(&a.CurrentPrice, whenNotDraft.WithRules(validation.Required, validation.By(func(any) error {
			v, _ := a.CurrentPrice.Float64()
			return validation.Min(0.00).Validate(v)
		}))),
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
