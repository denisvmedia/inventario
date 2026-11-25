package rules

import (
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"
)

var (
	// ErrConvertedPriceNotZero is the error that returns when the original price is in the group currency
	// but the converted original price is not zero.
	ErrConvertedPriceNotZero = validation.NewError(
		"validation_converted_price_not_zero",
		"converted original price must be zero when original price is in the group currency",
	)

	// ErrAllPricesZero is the error that returns when all prices are zero.
	ErrAllPricesZero = validation.NewError(
		"validation_all_prices_zero",
		"at least one of current price, original price, or converted original price must be set",
	)

	// ErrNoPriceInGroupCurrency is the error that returns when the original price is not in the group currency
	// and neither the converted original price nor the current price is set.
	ErrNoPriceInGroupCurrency = validation.NewError(
		"validation_no_price_in_group_currency",
		"if original price is not in the group currency, converted original price or current price must be set",
	)
)

// PriceRule checks the currency invariants between the three prices of a
// commodity. A nil price is unknown, which is distinct from a known zero, and
// the invariants only constrain known amounts.
type PriceRule struct {
	GroupCurrency    string
	OriginalCurrency string
	OriginalPrice    *decimal.Decimal
	ConvertedPrice   *decimal.Decimal
	CurrentPrice     *decimal.Decimal
}

// NewPriceRule creates a new PriceRule.
func NewPriceRule(groupCurrency, originalCurrency string, originalPrice, convertedPrice, currentPrice *decimal.Decimal) PriceRule {
	return PriceRule{
		GroupCurrency:    groupCurrency,
		OriginalCurrency: originalCurrency,
		ConvertedPrice:   convertedPrice,
		OriginalPrice:    originalPrice,
		CurrentPrice:     currentPrice,
	}
}

// Validate implements the validation.Rule interface.
// It checks the following conditions:
//  1. If the original price is in the group currency, a known converted price
//     must be zero: the original price already is the group-currency amount.
//  2. If the original price is known and not in the group currency, the
//     converted price or the current price must be known, so the item has an
//     amount in the group currency.
//
// An unknown original price needs no conversion, and an item may have no
// known price at all.
func (r PriceRule) Validate(_ any) error {
	if r.OriginalCurrency == r.GroupCurrency && r.ConvertedPrice != nil && !r.ConvertedPrice.IsZero() {
		return ErrConvertedPriceNotZero
	}

	if r.OriginalPrice != nil && r.OriginalCurrency != r.GroupCurrency && r.ConvertedPrice == nil && r.CurrentPrice == nil {
		return ErrNoPriceInGroupCurrency
	}

	return nil
}
