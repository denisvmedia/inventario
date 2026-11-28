package memory

import (
	"context"
)

// pickAssetNumber returns the asset number a new commodity of groupID gets:
// requested when it is positive and no commodity of the group holds it,
// otherwise the group's highest number plus one. The caller holds
// r.assetNumbers until the new row is stored, the in-memory counterpart of
// the postgres per-group advisory lock.
func (r *CommodityRegistry) pickAssetNumber(groupID string, requested int) int {
	r.lock.RLock()
	defer r.lock.RUnlock()

	top, taken := 0, false
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		c := pair.Value
		if c == nil || c.GroupID != groupID || c.AssetNumber == nil {
			continue
		}
		top = max(top, *c.AssetNumber)
		if *c.AssetNumber == requested {
			taken = true
		}
	}
	if requested > 0 && !taken {
		return requested
	}
	return top + 1
}

// BackfillAssetNumbers numbers the visible commodities that have no asset
// number. The map keeps insertion order, which is creation order here, so
// each group's numbers continue after its highest one in that order. See
// registry.CommodityRegistry for the contract.
func (r *CommodityRegistry) BackfillAssetNumbers(ctx context.Context) (int, error) {
	r.assetNumbers.Lock()
	defer r.assetNumbers.Unlock()
	r.lock.Lock()
	defer r.lock.Unlock()

	top := make(map[string]int)
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		c := pair.Value
		if c != nil && c.AssetNumber != nil {
			top[c.GroupID] = max(top[c.GroupID], *c.AssetNumber)
		}
	}

	numbered := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if err := ctx.Err(); err != nil {
			return numbered, err
		}
		c := pair.Value
		if c == nil || c.AssetNumber != nil || !r.isItemVisible(c) {
			continue
		}
		top[c.GroupID]++
		cp := *c
		cp.AssetNumber = new(top[c.GroupID])
		r.items.Set(pair.Key, &cp)
		numbered++
	}
	return numbered, nil
}

// BackfillLegacyPrices rewrites the rows flagged LegacyZeroPrices the way the
// postgres registry does: a zero current or converted price becomes unknown
// and the flag is cleared. Create and Update never store the flag, so in
// practice there is nothing to rewrite here.
func (r *CommodityRegistry) BackfillLegacyPrices(ctx context.Context) (int, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	rewritten := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if err := ctx.Err(); err != nil {
			return rewritten, err
		}
		c := pair.Value
		if c == nil || !c.LegacyZeroPrices || !r.isItemVisible(c) {
			continue
		}
		cp := *c
		if cp.CurrentPrice != nil && cp.CurrentPrice.IsZero() {
			cp.CurrentPrice = nil
		}
		if cp.ConvertedOriginalPrice != nil && cp.ConvertedOriginalPrice.IsZero() {
			cp.ConvertedOriginalPrice = nil
		}
		cp.LegacyZeroPrices = false
		cp.Revision++
		r.items.Set(pair.Key, &cp)
		rewritten++
	}
	return rewritten, nil
}

// BackfillRegisteredDates has nothing to date here: Create stores a
// registered date on every item and Update keeps it, so no item of this
// registry is without one. See registry.CommodityRegistry for the contract.
func (*CommodityRegistry) BackfillRegisteredDates(context.Context) (int, error) {
	return 0, nil
}
