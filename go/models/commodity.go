package models

import (
	"context"
	"encoding/json"
	"errors"

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
	_ IDable                            = (*Commodity)(nil)
	_ json.Marshaler                    = (*Commodity)(nil)
	_ json.Unmarshaler                  = (*Commodity)(nil)
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="commodities" comment="Enable RLS for multi-tenant commodity isolation"
//migrator:schema:rls:policy name="commodity_isolation" table="commodities" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures commodities can only be accessed and modified by their tenant and user with required contexts"
//migrator:schema:rls:policy name="commodity_background_worker_access" table="commodities" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all commodities for processing"

//migrator:schema:table name="commodities"
type Commodity struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
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
}

// PostgreSQL-specific indexes for commodities
type CommodityIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_commodities_tenant_id" fields="tenant_id" table="commodities"
	_ int

	// Composite index for tenant + area queries
	//migrator:schema:index name="idx_commodities_tenant_area" fields="tenant_id,area_id" table="commodities"
	_ int

	// Composite index for tenant + status queries
	//migrator:schema:index name="idx_commodities_tenant_status" fields="tenant_id,status" table="commodities"
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
}

func (*Commodity) Validate() error {
	return ErrMustUseValidateWithContext
}

func (a *Commodity) ValidateWithContext(ctx context.Context) error {
	mainCurrency, err := validationctx.MainCurrencyFromContext(ctx)
	if errors.Is(err, validationctx.ErrMainCurrencyNotSet) {
		return validation.NewError("main_currency_not_set", "main currency not set")
	}
	if err != nil {
		return err
	}

	fields := make([]*validation.FieldRules, 0)

	// Create a validation rule for price consistency
	priceRule := rules.NewPriceRule(
		string(mainCurrency),
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
