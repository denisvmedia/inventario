package rules_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models/rules"
)

func price(v int64) *decimal.Decimal {
	return new(decimal.NewFromInt(v))
}

func TestPriceRule_Validate_Valid(t *testing.T) {
	testCases := []struct {
		name          string
		origCurrency  string
		origPrice     *decimal.Decimal
		convertedPrc  *decimal.Decimal
		currentPrice  *decimal.Decimal
		groupCurrency string
	}{
		{name: "original price in group currency, converted unknown", origCurrency: "USD", origPrice: price(100)},
		{name: "original price in group currency, converted known zero", origCurrency: "USD", origPrice: price(100), convertedPrc: price(0)},
		{name: "foreign original price with converted price", origCurrency: "EUR", origPrice: price(100), convertedPrc: price(110)},
		{name: "foreign original price with current price only", origCurrency: "EUR", origPrice: price(100), currentPrice: price(110)},
		{name: "only current price known", origCurrency: "USD", currentPrice: price(110)},
		{name: "every price unknown", origCurrency: "USD"},
		{name: "every price unknown in a foreign currency", origCurrency: "EUR"},
		{name: "free acquisition in group currency", origCurrency: "USD", origPrice: price(0)},
		{name: "free acquisition in a foreign currency, converted known zero", origCurrency: "EUR", origPrice: price(0), convertedPrc: price(0)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			rule := rules.NewPriceRule("USD", tc.origCurrency, tc.origPrice, tc.convertedPrc, tc.currentPrice)
			c.Assert(rule.Validate(nil), qt.IsNil)
		})
	}
}

func TestPriceRule_Validate_Invalid(t *testing.T) {
	testCases := []struct {
		name         string
		origCurrency string
		origPrice    *decimal.Decimal
		convertedPrc *decimal.Decimal
		currentPrice *decimal.Decimal
		expectedErr  error
	}{
		{
			name:         "original price in group currency but converted price is not zero",
			origCurrency: "USD",
			origPrice:    price(100),
			convertedPrc: price(110),
			expectedErr:  rules.ErrConvertedPriceNotZero,
		},
		{
			name:         "unknown original price in group currency with a nonzero converted price",
			origCurrency: "USD",
			convertedPrc: price(110),
			expectedErr:  rules.ErrConvertedPriceNotZero,
		},
		{
			name:         "foreign original price with neither converted nor current price",
			origCurrency: "EUR",
			origPrice:    price(100),
			expectedErr:  rules.ErrNoPriceInGroupCurrency,
		},
		{
			name:         "foreign free acquisition still needs a group-currency amount",
			origCurrency: "EUR",
			origPrice:    price(0),
			expectedErr:  rules.ErrNoPriceInGroupCurrency,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			rule := rules.NewPriceRule("USD", tc.origCurrency, tc.origPrice, tc.convertedPrc, tc.currentPrice)
			err := rule.Validate(nil)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Equals, tc.expectedErr.Error())
		})
	}
}
