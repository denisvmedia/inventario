package jsonapi_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/jsonapi"
)

func costCode(c *qt.C, err error) string {
	c.Helper()
	var errs validation.Errors
	c.Assert(err, qt.ErrorAs, &errs)
	var fieldErr validation.Error
	c.Assert(errs["cost_currency"], qt.ErrorAs, &fieldErr)
	return fieldErr.Code()
}

func TestCommodityServiceRequestData_ValidateWithContext_Cost(t *testing.T) {
	tests := []struct {
		name     string
		cost     string
		wantCode string
	}{
		{name: "no cost", cost: ``},
		{name: "a cost with its currency", cost: `,"cost_amount":"245.50","cost_currency":"EUR"`},
		{name: "a cost of 0 with its currency", cost: `,"cost_amount":"0","cost_currency":"EUR"`},
		{name: "an amount of 0 without a currency is no cost", cost: `,"cost_amount":"0"`},
		{name: "an amount without a currency", cost: `,"cost_amount":"245.50"`, wantCode: "cost_currency_pair_required"},
		{name: "a currency without an amount", cost: `,"cost_currency":"EUR"`, wantCode: "cost_currency_pair_required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			var d jsonapi.CommodityServiceRequestData
			body := `{"provider_name":"Repair Shop","sent_at":"2026-05-01"` + tt.cost + `}`
			c.Assert(json.Unmarshal([]byte(body), &d), qt.IsNil)

			err := d.ValidateWithContext(context.Background())

			if tt.wantCode == "" {
				c.Assert(err, qt.IsNil)
				return
			}
			c.Assert(costCode(c, err), qt.Equals, tt.wantCode)
		})
	}
}

func TestCommodityServiceReturnRequestData_ValidateWithContext_Cost(t *testing.T) {
	tests := []struct {
		name     string
		cost     string
		wantCode string
	}{
		{name: "no cost", cost: `{}`},
		{name: "a cost of 0 with its currency", cost: `{"cost_amount":"0","cost_currency":"EUR"}`},
		{name: "an amount alone keeps the row's currency", cost: `{"cost_amount":"12"}`},
		{name: "a currency without an amount", cost: `{"cost_currency":"EUR"}`, wantCode: "cost_currency_pair_required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			var d jsonapi.CommodityServiceReturnRequestData
			c.Assert(json.Unmarshal([]byte(tt.cost), &d), qt.IsNil)

			err := d.ValidateWithContext(context.Background())

			if tt.wantCode == "" {
				c.Assert(err, qt.IsNil)
				return
			}
			c.Assert(costCode(c, err), qt.Equals, tt.wantCode)
		})
	}
}
