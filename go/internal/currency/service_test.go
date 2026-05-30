package currency_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/models"
)

// commodity is a minimal builder. We only set the fields ApplyConversion
// reads; the embedded TenantGroupAwareEntityID stays zero because the
// pure function never looks at IDs.
func commodity(originalPrice, convertedOriginalPrice, currentPrice string, originalCurrency models.Currency, ap *string, ac *models.Currency) models.Commodity {
	c := models.Commodity{
		OriginalPrice:          decimal.RequireFromString(originalPrice),
		OriginalPriceCurrency:  originalCurrency,
		ConvertedOriginalPrice: decimal.RequireFromString(convertedOriginalPrice),
		CurrentPrice:           decimal.RequireFromString(currentPrice),
	}
	if ap != nil {
		d := decimal.RequireFromString(*ap)
		c.AcquisitionPrice = &d
	}
	if ac != nil {
		cur := *ac
		c.AcquisitionCurrency = &cur
	}
	return c
}

func TestApplyConversion_CaseA_FillsAcquisitionWhenNull(t *testing.T) {
	c := qt.New(t)

	row := commodity("100", "0", "120", models.Currency("USD"), nil, nil)
	rate := decimal.RequireFromString("0.9")

	got := currency.ApplyConversion(row, "USD", "EUR", rate)

	c.Assert(got.Outcome, qt.Equals, currency.ApplyOutcomeCaseA)
	c.Assert(got.FillAcquisition, qt.IsTrue)
	c.Assert(got.AcquisitionPrice.String(), qt.Equals, "100")
	c.Assert(got.AcquisitionCurrency, qt.Equals, models.Currency("USD"))
	c.Assert(got.After.OriginalPrice.String(), qt.Equals, "90")
	c.Assert(got.After.OriginalPriceCurrency, qt.Equals, models.Currency("EUR"))
	c.Assert(got.After.ConvertedOriginalPrice.IsZero(), qt.IsTrue)
	c.Assert(got.After.CurrentPrice.String(), qt.Equals, "108")
}

func TestApplyConversion_CaseA_DoesNotRefillAcquisitionWhenAlreadySet(t *testing.T) {
	c := qt.New(t)

	preset := "42"
	presetCurrency := models.Currency("CAD")
	row := commodity("100", "0", "120", models.Currency("USD"), &preset, &presetCurrency)
	rate := decimal.RequireFromString("0.9")

	got := currency.ApplyConversion(row, "USD", "EUR", rate)

	// Case-A still applies (the live original was in G_old) — but the
	// caller must not re-fill acquisition columns. Once written, they
	// are frozen.
	c.Assert(got.Outcome, qt.Equals, currency.ApplyOutcomeCaseA)
	c.Assert(got.FillAcquisition, qt.IsFalse)
	c.Assert(got.After.OriginalPrice.String(), qt.Equals, "90")
}

func TestApplyConversion_CaseB_LeavesOriginalUntouchedScalesGSide(t *testing.T) {
	c := qt.New(t)

	// OriginalPriceCurrency=GBP, group migrating USD→EUR. Case B.
	row := commodity("3", "8", "11", models.Currency("GBP"), nil, nil)
	rate := decimal.RequireFromString("0.9")

	got := currency.ApplyConversion(row, "USD", "EUR", rate)

	c.Assert(got.Outcome, qt.Equals, currency.ApplyOutcomeCaseB)
	c.Assert(got.FillAcquisition, qt.IsFalse)
	c.Assert(got.After.OriginalPrice.String(), qt.Equals, "3")
	c.Assert(got.After.OriginalPriceCurrency, qt.Equals, models.Currency("GBP"))
	c.Assert(got.After.ConvertedOriginalPrice.String(), qt.Equals, "7.2")
	c.Assert(got.After.CurrentPrice.String(), qt.Equals, "9.9")
}

func TestApplyConversion_CaseC_CollapsesConvertedToZero(t *testing.T) {
	c := qt.New(t)

	// OriginalPriceCurrency already equals the *target* currency. The
	// previous service multiplied ConvertedOriginalPrice by the rate
	// and left the row in violation of PriceRule. The new
	// implementation collapses it to zero.
	row := commodity("100", "5", "120", models.Currency("EUR"), nil, nil)
	rate := decimal.RequireFromString("0.9")

	got := currency.ApplyConversion(row, "USD", "EUR", rate)

	c.Assert(got.Outcome, qt.Equals, currency.ApplyOutcomeCaseC)
	c.Assert(got.FillAcquisition, qt.IsFalse)
	c.Assert(got.After.OriginalPrice.String(), qt.Equals, "100")
	c.Assert(got.After.OriginalPriceCurrency, qt.Equals, models.Currency("EUR"))
	c.Assert(got.After.ConvertedOriginalPrice.IsZero(), qt.IsTrue)
	c.Assert(got.After.CurrentPrice.String(), qt.Equals, "108")
}

func TestApplyConversion_RoundsHalfAwayFromZero(t *testing.T) {
	c := qt.New(t)

	// shopspring/decimal Round defaults to half-away-from-zero: 12.345 → 12.35.
	row := commodity("10", "0", "5", models.Currency("USD"), nil, nil)
	rate := decimal.RequireFromString("1.23456")

	got := currency.ApplyConversion(row, "USD", "EUR", rate)

	c.Assert(got.Outcome, qt.Equals, currency.ApplyOutcomeCaseA)
	c.Assert(got.After.OriginalPrice.String(), qt.Equals, "12.35")
	c.Assert(got.After.CurrentPrice.String(), qt.Equals, "6.17")
}

func TestValidateRate(t *testing.T) {
	c := qt.New(t)

	c.Assert(currency.ValidateRate(decimal.RequireFromString("1")), qt.IsNil)
	c.Assert(currency.ValidateRate(decimal.RequireFromString("0.000001")), qt.IsNil)
	c.Assert(currency.ValidateRate(decimal.RequireFromString("10000000000")), qt.IsNil)

	// Reject zero, negative, and >1e10.
	c.Assert(currency.ValidateRate(decimal.Zero), qt.ErrorIs, currency.ErrInvalidExchangeRate)
	c.Assert(currency.ValidateRate(decimal.RequireFromString("-1")), qt.ErrorIs, currency.ErrInvalidExchangeRate)
	c.Assert(currency.ValidateRate(decimal.RequireFromString("10000000001")), qt.ErrorIs, currency.ErrInvalidExchangeRate)
}
