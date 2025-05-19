package rules_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models/rules"
)

func TestPriceRule_Validate(t *testing.T) {
	// Happy path tests
	t.Run("valid when original price is in main currency and converted price is zero", func(t *testing.T) {
		c := qt.New(t)
		rule := rules.NewPriceRule("USD", "USD", decimal.NewFromInt(100), decimal.Zero, decimal.Zero)
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNil)
	})

	t.Run("valid when original price is not in main currency and converted price is set", func(t *testing.T) {
		c := qt.New(t)
		rule := rules.NewPriceRule("USD", "EUR", decimal.NewFromInt(100), decimal.NewFromInt(110), decimal.Zero)
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNil)
	})

	t.Run("valid when original price is not in main currency and current price is set", func(t *testing.T) {
		c := qt.New(t)
		rule := rules.NewPriceRule("USD", "EUR", decimal.NewFromInt(100), decimal.Zero, decimal.NewFromInt(110))
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNil)
	})

	t.Run("valid when only current price is set", func(t *testing.T) {
		c := qt.New(t)
		rule := rules.NewPriceRule("USD", "USD", decimal.Zero, decimal.Zero, decimal.NewFromInt(110))
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNil)
	})

	t.Run("all prices are zero", func(t *testing.T) {
		c := qt.New(t)
		rule := rules.NewPriceRule("USD", "USD", decimal.Zero, decimal.Zero, decimal.Zero)
		err := rule.Validate(nil)
		c.Assert(err, qt.IsNil)
	})

	// Unhappy path tests
	t.Run("invalid cases", func(t *testing.T) {
		testCases := []struct {
			name           string
			mainCurrency   string
			origCurrency   string
			origPrice      decimal.Decimal
			convertedPrice decimal.Decimal
			currentPrice   decimal.Decimal
			expectedErr    error
		}{
			{
				name:           "original price in main currency but converted price is not zero",
				mainCurrency:   "USD",
				origCurrency:   "USD",
				origPrice:      decimal.NewFromInt(100),
				convertedPrice: decimal.NewFromInt(110),
				currentPrice:   decimal.Zero,
				expectedErr:    rules.ErrConvertedPriceNotZero,
			},
			// {
			//	name:           "all prices are zero",
			//	mainCurrency:   "USD",
			//	origCurrency:   "USD",
			//	origPrice:      decimal.Zero,
			//	convertedPrice: decimal.Zero,
			//	currentPrice:   decimal.Zero,
			//	expectedErr:    rules.ErrAllPricesZero,
			// },
			{
				name:           "original price not in main currency and neither converted nor current price is set",
				mainCurrency:   "USD",
				origCurrency:   "EUR",
				origPrice:      decimal.NewFromInt(100),
				convertedPrice: decimal.Zero,
				currentPrice:   decimal.Zero,
				expectedErr:    rules.ErrNoPriceInMainCurrency,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				rule := rules.NewPriceRule(tc.mainCurrency, tc.origCurrency, tc.origPrice, tc.convertedPrice, tc.currentPrice)
				err := rule.Validate(nil)
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Equals, tc.expectedErr.Error())
			})
		}
	})
}
