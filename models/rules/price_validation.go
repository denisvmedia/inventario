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
)

// ConvertedPriceRule validates that when the original price is in the main currency,
// the converted original price must be zero.
type ConvertedPriceRule struct {
	MainCurrency     string
	OriginalCurrency string
	ConvertedPrice   decimal.Decimal
}

// NewConvertedPriceRule creates a new ConvertedPriceRule.
func NewConvertedPriceRule(mainCurrency, originalCurrency string, convertedPrice decimal.Decimal) ConvertedPriceRule {
	return ConvertedPriceRule{
		MainCurrency:     mainCurrency,
		OriginalCurrency: originalCurrency,
		ConvertedPrice:   convertedPrice,
	}
}

// Validate implements the validation.Rule interface.
func (r ConvertedPriceRule) Validate(_ any) error {
	// If original currency is the main currency and converted price is not zero, return error
	if r.OriginalCurrency == r.MainCurrency && !r.ConvertedPrice.IsZero() {
		return ErrConvertedPriceNotZero
	}
	return nil
}
