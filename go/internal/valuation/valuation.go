// Package valuation provides functionality for calculating the total value of commodities.
package valuation

import (
	"context"

	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// Total is the value of a set of commodities in the group currency, with the
// coverage behind it. Value sums the commodities whose value is known (a known
// zero included); UnvaluedCount counts the commodities that were left out
// because they have no usable value, so a reader can tell a total that covers
// everything from one that covers part of the inventory.
type Total struct {
	Value         decimal.Decimal
	ValuedCount   int
	UnvaluedCount int
}

// Totals is the valuation of the active group: globally, by location and by
// area. Only commodities that count towards the valuation are included: drafts
// and commodities that are not in use are left out of every total and of the
// coverage counts. A commodity without an area counts towards the global total
// only.
type Totals struct {
	Global     Total
	ByLocation map[string]Total
	ByArea     map[string]Total
}

// Valuator provides methods for calculating commodity values.
type Valuator struct {
	CommodityRegistry registry.CommodityRegistry
	AreaRegistry      registry.AreaRegistry
	LocationRegistry  registry.LocationRegistry
	ctx               context.Context
}

// NewValuator creates a new Valuator instance. Pass a context carrying the
// group whose valuation you want (via appctx.WithGroup); GetGroupCurrency
// falls back to USD when no group is on the context or the group's currency
// is empty, which is a safe default for the valuation math but NOT a
// replacement for producing the right totals — callers that care should
// stamp the group themselves.
func NewValuator(ctx context.Context, registrySet *registry.Set) *Valuator {
	return &Valuator{
		CommodityRegistry: registrySet.CommodityRegistry,
		AreaRegistry:      registrySet.AreaRegistry,
		LocationRegistry:  registrySet.LocationRegistry,
		ctx:               ctx,
	}
}

// GetGroupCurrency returns the group currency of the group in context, falling
// back to USD when no group is attached or the group's currency is empty.
func (v *Valuator) GetGroupCurrency() (string, error) {
	group := appctx.GroupFromContext(v.ctx)
	if group == nil || group.GroupCurrency == "" {
		return "USD", nil
	}
	return string(group.GroupCurrency), nil
}

// Calculate computes the valuation of the group in one pass over its
// commodities. Each commodity's value comes from models.Commodity.Value: the
// current price when known, otherwise the purchase price in the group
// currency. A commodity with neither is counted in UnvaluedCount of every
// total it belongs to instead of being added as zero. Prices already describe
// the whole lot, so the count of units does not multiply them.
func (v *Valuator) Calculate() (Totals, error) {
	ctx := v.ctx

	groupCurrency, err := v.GetGroupCurrency()
	if err != nil {
		return Totals{}, err
	}

	commodities, err := v.CommodityRegistry.List(ctx)
	if err != nil {
		return Totals{}, err
	}

	areas, err := v.AreaRegistry.List(ctx)
	if err != nil {
		return Totals{}, err
	}
	areaToLocation := make(map[string]string, len(areas))
	for _, area := range areas {
		areaToLocation[area.ID] = area.LocationID
	}

	totals := Totals{
		ByLocation: make(map[string]Total),
		ByArea:     make(map[string]Total),
	}
	for _, commodity := range commodities {
		if commodity.Draft || commodity.Status != models.CommodityStatusInUse {
			continue
		}

		value, known := commodity.Value(models.Currency(groupCurrency))
		count := func(t Total) Total {
			if !known {
				t.UnvaluedCount++
				return t
			}
			t.Value = t.Value.Add(value)
			t.ValuedCount++
			return t
		}
		totals.Global = count(totals.Global)

		// Area is optional (issue #1986): an unassigned commodity has no area
		// or location bucket.
		if commodity.AreaID == nil {
			continue
		}
		totals.ByArea[*commodity.AreaID] = count(totals.ByArea[*commodity.AreaID])

		locationID, ok := areaToLocation[*commodity.AreaID]
		if !ok {
			continue
		}
		totals.ByLocation[locationID] = count(totals.ByLocation[locationID])
	}

	return totals, nil
}
