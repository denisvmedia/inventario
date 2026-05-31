package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// TestCommodityRegistry_ListPaginated_LentOutFilter pins the LentOut +
// OpenLoanCommodityIDs wiring on CommodityListOptions. The memory backend
// is decoupled from CommodityLoanRegistry — the apiserver pre-resolves
// the open-loan commodity ID set and feeds it through opts. The filter
// MUST treat absent IDs as not-lent (true filter excludes them) and
// present IDs as lent (false filter excludes them). Edge case: a commodity
// listed in OpenLoanCommodityIDs that has multiple historical loans (older
// returned, newer open) is still "currently lent" and survives the
// `LentOut=true` filter — this matches the postgres EXISTS subquery
// (which fires on any open row, regardless of age).
func TestCommodityRegistry_ListPaginated_LentOutFilter(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID := newCommodityWarrantyFixture(c)

	mk := func(name string) *models.Commodity {
		c.Helper()
		out, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
			AreaID:    new(areaID),
			Name:      name,
			ShortName: name,
			Status:    models.CommodityStatusInUse,
			Type:      models.CommodityTypeOther,
			Count:     1,
		})
		c.Assert(err, qt.IsNil)
		return out
	}

	lent := mk("lent-out")
	mk("at-home")
	multiOpen := mk("multi-open") // simulates an item with an old returned loan + a newer open one

	// Caller (apiserver in production) pre-resolves the set of currently
	// lent commodity IDs. We pass two of three.
	openIDs := []string{lent.ID, multiOpen.ID}
	tru := true
	fls := false

	tests := []struct {
		name   string
		opts   registry.CommodityListOptions
		expect []string
	}{
		{
			name: "lent_out=true returns only the open-loan commodities",
			opts: registry.CommodityListOptions{
				IncludeInactive:      true,
				LentOut:              &tru,
				OpenLoanCommodityIDs: openIDs,
			},
			expect: []string{"lent-out", "multi-open"},
		},
		{
			name: "lent_out=false returns only the not-lent commodities",
			opts: registry.CommodityListOptions{
				IncludeInactive:      true,
				LentOut:              &fls,
				OpenLoanCommodityIDs: openIDs,
			},
			expect: []string{"at-home"},
		},
		{
			name: "lent_out nil leaves results unfiltered",
			opts: registry.CommodityListOptions{
				IncludeInactive: true,
			},
			expect: []string{"at-home", "lent-out", "multi-open"},
		},
		{
			name: "lent_out=true with empty open-set returns nothing",
			opts: registry.CommodityListOptions{
				IncludeInactive:      true,
				LentOut:              &tru,
				OpenLoanCommodityIDs: nil,
			},
			expect: []string{},
		},
		{
			name: "lent_out=false with empty open-set returns everything",
			opts: registry.CommodityListOptions{
				IncludeInactive:      true,
				LentOut:              &fls,
				OpenLoanCommodityIDs: nil,
			},
			expect: []string{"at-home", "lent-out", "multi-open"},
		},
	}

	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			items, _, err := regSet.CommodityRegistry.ListPaginated(ctx, 0, 100, tc.opts)
			c.Assert(err, qt.IsNil)
			names := make([]string, len(items))
			for i, it := range items {
				names[i] = it.Name
			}
			c.Assert(names, qt.ContentEquals, tc.expect)
		})
	}
}
