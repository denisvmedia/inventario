package models_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestSupplyLink_Validate_RejectsContextlessValidate(t *testing.T) {
	c := qt.New(t)
	// SupplyLink ships ValidateWithContext only — Validate() must reject
	// the context-less call so callers can't accidentally bypass the
	// URL parse the context flavour runs.
	c.Assert((&models.SupplyLink{}).Validate(), qt.Equals, models.ErrMustUseValidateWithContext)
}

func TestSupplyLink_ValidateWithContext_HappyPath(t *testing.T) {
	c := qt.New(t)
	link := models.SupplyLink{
		CommodityID: "commodity-1",
		Label:       "Water filter",
		URL:         "https://example.com/water-filter",
		Notes:       "Pack of 2, lasts ~6mo",
	}
	c.Assert(link.ValidateWithContext(context.Background()), qt.IsNil)
}

func TestSupplyLink_ValidateWithContext_UnhappyPath(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*models.SupplyLink)
		want string
	}{
		{
			name: "label empty",
			mut:  func(l *models.SupplyLink) { l.Label = "" },
			want: "label",
		},
		{
			name: "label too long",
			mut:  func(l *models.SupplyLink) { l.Label = strings.Repeat("x", 201) },
			want: "label",
		},
		{
			name: "url empty",
			mut:  func(l *models.SupplyLink) { l.URL = "" },
			want: "url",
		},
		{
			name: "url missing scheme",
			mut:  func(l *models.SupplyLink) { l.URL = "example.com/refill" },
			want: "url",
		},
		{
			name: "url ftp not allowed",
			mut:  func(l *models.SupplyLink) { l.URL = "ftp://example.com/refill" },
			want: "url",
		},
		{
			name: "url missing host",
			mut:  func(l *models.SupplyLink) { l.URL = "https:///path" },
			want: "url",
		},
		{
			name: "notes too long",
			mut:  func(l *models.SupplyLink) { l.Notes = strings.Repeat("y", 1001) },
			want: "notes",
		},
		{
			name: "commodity_id empty",
			mut:  func(l *models.SupplyLink) { l.CommodityID = "" },
			want: "commodity_id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			link := models.SupplyLink{
				CommodityID: "commodity-1",
				Label:       "Water filter",
				URL:         "https://example.com/water-filter",
				Notes:       "ok",
			}
			tc.mut(&link)
			err := link.ValidateWithContext(context.Background())
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.want)
		})
	}
}
