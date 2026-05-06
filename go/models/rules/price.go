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

// PriceRule validates that when the original price is in the group currency,
// the converted original price must be zero.
type PriceRule struct {
	GroupCurrency    string
	OriginalCurrency string
	OriginalPrice    decimal.Decimal
	ConvertedPrice   decimal.Decimal
	CurrentPrice     decimal.Decimal
}

// NewPriceRule creates a new PriceRule.
func NewPriceRule(groupCurrency, originalCurrency string, originalPrice, convertedPrice, currentPrice decimal.Decimal) PriceRule {
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
// 1. If the original price is in the group currency, the converted original price must be zero.
// 2. At least one of the prices (current, original, or converted original) must be set.
// 3. If the original price is not in the group currency, either the converted original price or the current price must be set.
func (r PriceRule) Validate(_ any) error {
	// If original currency is the group currency and converted price is not zero, return error
	if r.OriginalCurrency == r.GroupCurrency && !r.ConvertedPrice.IsZero() {
		return ErrConvertedPriceNotZero
	}

	// Allow all zeroes (the commodity is not counted as valuable)
	// If all prices are zero, return error
	// if r.CurrentPrice.IsZero() && r.OriginalPrice.IsZero() && r.ConvertedPrice.IsZero() {
	//	return ErrAllPricesZero
	// }

	// If original currency is not the group currency and neither converted price nor current price is set, return error
	if r.OriginalCurrency != r.GroupCurrency && r.ConvertedPrice.IsZero() && r.CurrentPrice.IsZero() {
		return ErrNoPriceInGroupCurrency
	}

	return nil
}
