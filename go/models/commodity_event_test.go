package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestCommodityEventKind_IsValid(t *testing.T) {
	// Locks the IsValid switch against future enum additions: a new
	// constant added without an entry in the switch will fail this
	// table because IsValid would return false.
	cases := []struct {
		kind models.CommodityEventKind
		want bool
	}{
		{models.CommodityEventKindCreated, true},
		{models.CommodityEventKindUpdated, true},
		{models.CommodityEventKindStatusChanged, true},
		{models.CommodityEventKindMoved, true},
		{models.CommodityEventKindPriceChanged, true},
		{models.CommodityEventKindCoverChanged, true},
		{models.CommodityEventKindLentOut, true},
		{models.CommodityEventKindReturned, true},
		{models.CommodityEventKindLoanUpdated, true},
		{models.CommodityEventKindDeleted, true},
		{"", false},
		{"unknown", false},
		{"LENT_OUT", false}, // case-sensitive — wire format is lowercase
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			c := qt.New(t)
			c.Assert(tc.kind.IsValid(), qt.Equals, tc.want)
		})
	}
}

func TestCommodityEventKind_Validate(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.CommodityEventKindLentOut.Validate(), qt.IsNil)
	c.Assert(models.CommodityEventKindReturned.Validate(), qt.IsNil)
	c.Assert(models.CommodityEventKindLoanUpdated.Validate(), qt.IsNil)

	err := models.CommodityEventKind("nope").Validate()
	c.Assert(err, qt.IsNotNil)
}
