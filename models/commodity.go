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
	CommodyTypeOther         CommodityType = "other"
)

func (c CommodityType) IsValid() bool {
	switch c {
	case CommodityTypeWhiteGoods,
		CommodityTypeElectronics,
		CommodityTypeEquipment,
		CommodityTypeFurniture,
		CommodityTypeClothes,
		CommodyTypeOther:
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
	Name                   string          `json:"name"`
	ShortName              string          `json:"short_name"`
	Type                   CommodityType   `json:"type"`
	AreaID                 string          `json:"area_id"`
	Count                  int             `json:"count"`
	OriginalPrice          decimal.Decimal `json:"original_price"`
	OriginalPriceCurrency  Currency        `json:"original_price_currency"`
	ConvertedOriginalPrice decimal.Decimal `json:"converted_original_price"`
	CurrentPrice           decimal.Decimal `json:"current_price"`
	SerialNumber           string          `json:"serial_number"`
	ExtraSerialNumbers     []string        `json:"extra_serial_numbers"`
	PartNumbers            []string        `json:"part_numbers"`
	Tags                   []string        `json:"tags"`
	Status                 CommodityStatus `json:"status"`
	PurchaseDate           PDate           `json:"purchase_date"`
	RegisteredDate         PDate           `json:"registered_date"`
	LastModifiedDate       PDate           `json:"last_modified_date"`
	URLs                   []*URL          `json:"urls" swaggertype:"string"`
	Comments               string          `json:"comments"`
	Draft                  bool            `json:"draft"`
}

func (a *Commodity) Validate() error {
	return validation.NewError("must_use_validate_with_context", "must use validate with context")
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
	// When original price is in the main currency, converted original price must be zero
	priceRule := rules.NewConvertedPriceRule(
		string(mainCurrency),
		string(a.OriginalPriceCurrency),
		a.ConvertedOriginalPrice,
	)

	fields = append(fields,
		validation.Field(&a.Name, rules.NotEmpty),
		validation.Field(&a.ShortName, rules.NotEmpty, validation.Length(1, 20)),
		validation.Field(&a.Type, rules.NotEmpty),
		validation.Field(&a.AreaID, rules.NotEmpty),
		validation.Field(&a.Status, rules.NotEmpty),
		validation.Field(&a.PurchaseDate, rules.NotEmpty),
		validation.Field(&a.Count, validation.Required, validation.Min(1)),
		validation.Field(&a.URLs),
		// Add validation for converted original price
		validation.Field(&a.ConvertedOriginalPrice, priceRule),
	)

	return validation.ValidateStruct(a, fields...)
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
