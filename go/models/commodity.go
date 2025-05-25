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

type Commodity struct {
	EntityID
	Name                   string          `json:"name" db:"name"`
	ShortName              string          `json:"short_name" db:"short_name"`
	Type                   CommodityType   `json:"type" db:"type"`
	AreaID                 string          `json:"area_id" db:"area_id"`
	Count                  int             `json:"count" db:"count"`
	OriginalPrice          decimal.Decimal `json:"original_price" db:"original_price"`
	OriginalPriceCurrency  Currency        `json:"original_price_currency" db:"original_price_currency"`
	ConvertedOriginalPrice decimal.Decimal `json:"converted_original_price" db:"converted_original_price"`
	CurrentPrice           decimal.Decimal `json:"current_price" db:"current_price"`
	SerialNumber           string          `json:"serial_number" db:"serial_number"`
	ExtraSerialNumbers     []string        `json:"extra_serial_numbers" db:"extra_serial_numbers"`
	PartNumbers            []string        `json:"part_numbers" db:"part_numbers"`
	Tags                   []string        `json:"tags" db:"tags"`
	Status                 CommodityStatus `json:"status" db:"status"`
	PurchaseDate           PDate           `json:"purchase_date" db:"purchase_date"`
	RegisteredDate         PDate           `json:"registered_date" db:"registered_date"`
	LastModifiedDate       PDate           `json:"last_modified_date" db:"last_modified_date"`
	URLs                   []*URL          `json:"urls" swaggertype:"string" db:"urls"`
	Comments               string          `json:"comments" db:"comments"`
	Draft                  bool            `json:"draft" db:"draft"`
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
