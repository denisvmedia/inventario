package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models"
)

// reviewedCommodity is a non-draft commodity with nothing known about its
// purchase: no date, no price.
func reviewedCommodity() models.Commodity {
	return models.Commodity{
		Name:                  "Grandfather clock",
		ShortName:             "Clock",
		Type:                  models.CommodityTypeFurniture,
		Count:                 1,
		Status:                models.CommodityStatusInUse,
		OriginalPriceCurrency: "USD",
	}
}

func TestCommodity_ValidateWithContext_UnknownPurchaseInfo(t *testing.T) {
	ctx := validationctx.WithGroupCurrency(context.Background(), "USD")

	testCases := []struct {
		name   string
		modify func(*models.Commodity)
	}{
		{
			name:   "gift with unknown date and price",
			modify: func(*models.Commodity) {},
		},
		{
			name: "gift in a foreign currency needs no conversion",
			modify: func(c *models.Commodity) {
				c.OriginalPriceCurrency = "EUR"
			},
		},
		{
			name: "free acquisition",
			modify: func(c *models.Commodity) {
				c.OriginalPrice = new(decimal.Zero)
				c.OriginalPriceCurrency = "USD"
				c.PurchaseDate = models.ToPDate("2024-05-01")
			},
		},
		{
			name: "only a current value",
			modify: func(c *models.Commodity) {
				c.CurrentPrice = new(decimal.NewFromInt(1200))
			},
		},
		{
			name: "known zero current value",
			modify: func(c *models.Commodity) {
				c.CurrentPrice = new(decimal.Zero)
			},
		},
		{
			name: "foreign-currency known price with its conversion",
			modify: func(c *models.Commodity) {
				c.OriginalPrice = new(decimal.NewFromInt(100))
				c.OriginalPriceCurrency = "EUR"
				c.ConvertedOriginalPrice = new(decimal.NewFromInt(108))
				c.PurchaseDate = models.ToPDate("2024-05-01")
			},
		},
		{
			name: "foreign-currency known price with a current value only",
			modify: func(c *models.Commodity) {
				c.OriginalPrice = new(decimal.NewFromInt(100))
				c.OriginalPriceCurrency = "EUR"
				c.CurrentPrice = new(decimal.NewFromInt(90))
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			commodity := reviewedCommodity()
			tc.modify(&commodity)
			c.Assert(commodity.ValidateWithContext(ctx), qt.IsNil)
		})
	}
}

func TestCommodity_ValidateWithContext_KnownPriceInvariants(t *testing.T) {
	ctx := validationctx.WithGroupCurrency(context.Background(), "USD")

	testCases := []struct {
		name          string
		modify        func(*models.Commodity)
		errorContains string
	}{
		{
			name: "missing currency",
			modify: func(c *models.Commodity) {
				c.OriginalPriceCurrency = ""
			},
			errorContains: "original_price_currency",
		},
		{
			name: "foreign-currency known price with no group-currency amount",
			modify: func(c *models.Commodity) {
				c.OriginalPrice = new(decimal.NewFromInt(100))
				c.OriginalPriceCurrency = "EUR"
			},
			errorContains: "converted original price or current price must be set",
		},
		{
			name: "negative current value",
			modify: func(c *models.Commodity) {
				c.CurrentPrice = new(decimal.NewFromInt(-1))
			},
			errorContains: "current_price",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			commodity := reviewedCommodity()
			tc.modify(&commodity)
			err := commodity.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestCommodity_JSON_KeepsPriceKnowledge(t *testing.T) {
	testCases := []struct {
		name  string
		price *decimal.Decimal
		json  string
	}{
		{name: "unknown", price: nil, json: `null`},
		{name: "zero", price: new(decimal.Zero), json: `"0"`},
		{name: "positive", price: new(decimal.RequireFromString("19.99")), json: `"19.99"`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			commodity := reviewedCommodity()
			commodity.OriginalPrice = tc.price
			commodity.ConvertedOriginalPrice = tc.price
			commodity.CurrentPrice = tc.price

			data, err := json.Marshal(commodity)
			c.Assert(err, qt.IsNil)

			var fields map[string]json.RawMessage
			c.Assert(json.Unmarshal(data, &fields), qt.IsNil)
			for _, name := range []string{"original_price", "converted_original_price", "current_price"} {
				c.Assert(string(fields[name]), qt.Equals, tc.json, qt.Commentf("%s", name))
			}
			c.Assert(string(fields["purchase_date"]), qt.Equals, `null`)

			var decoded models.Commodity
			c.Assert(json.Unmarshal(data, &decoded), qt.IsNil)
			c.Assert(models.SamePrice(decoded.OriginalPrice, tc.price), qt.IsTrue)
			c.Assert(models.SamePrice(decoded.ConvertedOriginalPrice, tc.price), qt.IsTrue)
			c.Assert(models.SamePrice(decoded.CurrentPrice, tc.price), qt.IsTrue)
			c.Assert(decoded.PurchaseDate, qt.IsNil)
		})
	}
}

func TestCommodity_Value(t *testing.T) {
	testCases := []struct {
		name      string
		commodity models.Commodity
		want      string
		known     bool
	}{
		{
			name:      "nothing known",
			commodity: models.Commodity{OriginalPriceCurrency: "USD"},
		},
		{
			name:      "free acquisition is a known zero",
			commodity: models.Commodity{OriginalPrice: new(decimal.Zero), OriginalPriceCurrency: "USD"},
			want:      "0",
			known:     true,
		},
		{
			name: "current value wins over the purchase price",
			commodity: models.Commodity{
				OriginalPrice: new(decimal.NewFromInt(100)), OriginalPriceCurrency: "USD",
				CurrentPrice: new(decimal.NewFromInt(80)),
			},
			want:  "80",
			known: true,
		},
		{
			name: "a known zero current value wins over the purchase price",
			commodity: models.Commodity{
				OriginalPrice: new(decimal.NewFromInt(100)), OriginalPriceCurrency: "USD",
				CurrentPrice: new(decimal.Zero),
			},
			want:  "0",
			known: true,
		},
		{
			name: "foreign purchase price uses its conversion",
			commodity: models.Commodity{
				OriginalPrice: new(decimal.NewFromInt(100)), OriginalPriceCurrency: "EUR",
				ConvertedOriginalPrice: new(decimal.NewFromInt(110)),
			},
			want:  "110",
			known: true,
		},
		{
			name: "foreign purchase price without a conversion has no value",
			commodity: models.Commodity{
				OriginalPrice: new(decimal.NewFromInt(100)), OriginalPriceCurrency: "EUR",
			},
		},
		{
			// What clients written before #2813 send for "not entered".
			name: "group-currency item ignores a stored converted zero",
			commodity: models.Commodity{
				OriginalPriceCurrency: "USD", ConvertedOriginalPrice: new(decimal.Zero),
			},
		},
		{
			name: "group-currency item ignores a stored converted price",
			commodity: models.Commodity{
				OriginalPriceCurrency: "USD", ConvertedOriginalPrice: new(decimal.NewFromInt(5)),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			value, known := tc.commodity.Value("USD")
			c.Assert(known, qt.Equals, tc.known)
			if tc.known {
				c.Assert(value.String(), qt.Equals, tc.want)
			}
		})
	}
}

func TestCommodity_NormalizePrices(t *testing.T) {
	c := qt.New(t)

	same := models.Commodity{OriginalPriceCurrency: "USD", ConvertedOriginalPrice: new(decimal.Zero)}
	same.NormalizePrices("USD")
	c.Assert(same.ConvertedOriginalPrice, qt.IsNil)

	foreign := models.Commodity{OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: new(decimal.Zero)}
	foreign.NormalizePrices("USD")
	c.Assert(foreign.ConvertedOriginalPrice, qt.IsNotNil)
	c.Assert(foreign.ConvertedOriginalPrice.IsZero(), qt.IsTrue)

	noGroupCurrency := models.Commodity{OriginalPriceCurrency: "USD", ConvertedOriginalPrice: new(decimal.Zero)}
	noGroupCurrency.NormalizePrices("")
	c.Assert(noGroupCurrency.ConvertedOriginalPrice, qt.IsNotNil)
}

func TestCommodity_UnsetEchoedLegacyZeros(t *testing.T) {
	legacy := models.Commodity{
		OriginalPriceCurrency:  "EUR",
		ConvertedOriginalPrice: new(decimal.Zero),
		CurrentPrice:           new(decimal.Zero),
		LegacyZeroPrices:       true,
	}
	testCases := []struct {
		name          string
		stored        *models.Commodity
		incoming      models.Commodity
		wantCurrent   string // "" means unknown
		wantConverted string
	}{
		{
			name:     "echoed zeros on a flagged row become unknown",
			stored:   new(legacy),
			incoming: models.Commodity{CurrentPrice: new(decimal.Zero), ConvertedOriginalPrice: new(decimal.Zero)},
		},
		{
			name:          "a price the caller changed is kept",
			stored:        new(legacy),
			incoming:      models.Commodity{CurrentPrice: new(decimal.NewFromInt(7)), ConvertedOriginalPrice: new(decimal.NewFromInt(9))},
			wantCurrent:   "7",
			wantConverted: "9",
		},
		{
			name: "zeros on a row the backfill already cleared are known zeros",
			stored: &models.Commodity{
				ConvertedOriginalPrice: new(decimal.Zero),
				CurrentPrice:           new(decimal.Zero),
			},
			incoming:      models.Commodity{CurrentPrice: new(decimal.Zero), ConvertedOriginalPrice: new(decimal.Zero)},
			wantCurrent:   "0",
			wantConverted: "0",
		},
		{
			name: "a zero over a stored non-zero price is the caller's own",
			stored: &models.Commodity{
				ConvertedOriginalPrice: new(decimal.NewFromInt(3)),
				CurrentPrice:           new(decimal.NewFromInt(3)),
				LegacyZeroPrices:       true,
			},
			incoming:      models.Commodity{CurrentPrice: new(decimal.Zero), ConvertedOriginalPrice: new(decimal.Zero)},
			wantCurrent:   "0",
			wantConverted: "0",
		},
		{
			name:     "no stored row",
			incoming: models.Commodity{CurrentPrice: new(decimal.Zero)},
			// Nothing to compare with: the zero is kept.
			wantCurrent: "0",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got := tc.incoming
			got.UnsetEchoedLegacyZeros(tc.stored)
			assertPrice(c, got.CurrentPrice, tc.wantCurrent)
			assertPrice(c, got.ConvertedOriginalPrice, tc.wantConverted)
		})
	}
}

func assertPrice(c *qt.C, got *decimal.Decimal, want string) {
	c.Helper()
	if want == "" {
		c.Assert(got, qt.IsNil)
		return
	}
	c.Assert(got, qt.IsNotNil)
	c.Assert(got.String(), qt.Equals, want)
}

func TestSamePrice(t *testing.T) {
	c := qt.New(t)
	c.Assert(models.SamePrice(nil, nil), qt.IsTrue)
	c.Assert(models.SamePrice(nil, new(decimal.Zero)), qt.IsFalse)
	c.Assert(models.SamePrice(new(decimal.Zero), nil), qt.IsFalse)
	c.Assert(models.SamePrice(new(decimal.RequireFromString("17500.00")), new(decimal.NewFromInt(17500))), qt.IsTrue)
	c.Assert(models.SamePrice(new(decimal.NewFromInt(1)), new(decimal.NewFromInt(2))), qt.IsFalse)
}
