package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
)

func TestCommodityService_ValidateWithContext_Cost(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		currency string
		wantCode string
		wantCost bool
	}{
		{name: "no cost", amount: "0", currency: ""},
		{name: "a cost with its currency", amount: "245.50", currency: "EUR", wantCost: true},
		{name: "a cost of 0 with its currency is a free repair", amount: "0", currency: "EUR", wantCost: true},
		{name: "an amount without a currency", amount: "245.50", currency: "", wantCode: "cost_currency_pair_required"},
		{name: "a currency that is not ISO 4217", amount: "0", currency: "EURO", wantCode: "cost_currency_iso_4217"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			svc := models.CommodityService{
				CommodityID:  "c-1",
				ProviderName: "Repair Shop",
				SentAt:       models.Date("2026-05-01"),
				CostAmount:   decimal.RequireFromString(tt.amount),
				CostCurrency: tt.currency,
			}

			err := svc.ValidateWithContext(context.Background())

			if tt.wantCode == "" {
				c.Assert(err, qt.IsNil)
				c.Assert(svc.HasCost(), qt.Equals, tt.wantCost)
				return
			}
			var errs validation.Errors
			c.Assert(err, qt.ErrorAs, &errs)
			c.Assert(errs, qt.HasLen, 1)
			var fieldErr validation.Error
			c.Assert(errs["cost_amount"], qt.ErrorAs, &fieldErr)
			c.Assert(fieldErr.Code(), qt.Equals, tt.wantCode)
		})
	}
}
