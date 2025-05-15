package rules

import (
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"
)

var (
	// ErrConvertedPriceNotZero is the error that returns when the original price is in the main currency
	// but the converted original price is not zero.
	ErrConvertedPriceNotZero = validation.NewError(
		"validation_converted_price_not_zero",
		"converted original price must be zero when original price is in the main currency",
	)

	// ErrAllPricesZero is the error that returns when all prices are zero.
	ErrAllPricesZero = validation.NewError(
		"validation_all_prices_zero",
		"at least one of current price, original price, or converted original price must be set",
	)

	// ErrNoPriceInMainCurrency is the error that returns when the original price is not in the main currency
	// and neither the converted original price nor the current price is set.
	ErrNoPriceInMainCurrency = validation.NewError(
		"validation_no_price_in_main_currency",
		"if original price is not in the main currency, converted original price or current price must be set",
	)
)

// ConvertedPriceRule validates that when the original price is in the main currency,
// the converted original price must be zero.
type ConvertedPriceRule struct {
	MainCurrency     string
	OriginalCurrency string
	OriginalPrice    decimal.Decimal
	ConvertedPrice   decimal.Decimal
	CurrentPrice     decimal.Decimal
}

// NewPriceRule creates a new ConvertedPriceRule.
func NewPriceRule(mainCurrency, originalCurrency string, originalPrice, convertedPrice, currentPrice decimal.Decimal) ConvertedPriceRule {
	return ConvertedPriceRule{
		MainCurrency:     mainCurrency,
		OriginalCurrency: originalCurrency,
		ConvertedPrice:   convertedPrice,
		OriginalPrice:    originalPrice,
		CurrentPrice:     currentPrice,
	}
}

// Validate implements the validation.Rule interface.
func (r ConvertedPriceRule) Validate(_ any) error {
	// If original currency is the main currency and converted price is not zero, return error
	if r.OriginalCurrency == r.MainCurrency && !r.ConvertedPrice.IsZero() {
		return ErrConvertedPriceNotZero
	}

	if r.CurrentPrice.IsZero() && r.OriginalPrice.IsZero() && r.ConvertedPrice.IsZero() {
		return ErrAllPricesZero
	}

	if r.OriginalCurrency != r.MainCurrency && r.ConvertedPrice.IsZero() && r.CurrentPrice.IsZero() {
		return ErrNoPriceInMainCurrency
	}

	return nil
}
