package services

import (
	"context"
	"errors"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// commodityRewriteAttempts bounds how often RewriteCommodity reads an item
// again after another write landed between its read and its write.
const commodityRewriteAttempts = 3

// RewriteCommodity changes part of an item and writes the whole item back
// with the revision it was read at (registry.WithExpectedRevision), so a
// write that landed in between, such as a bulk change or a tag rename, is
// not undone. When the item changed since the read, the registry refuses
// the write with registry.ErrRevisionConflict; RewriteCommodity then reads
// the item again and applies change to that read, up to three times in
// all, and returns the conflict after the last.
//
// read, when not nil, is the caller's own read of the item and saves the
// first Get. change edits the copy it is given and reports whether there is
// anything to write; when it reports false nothing is written, after is
// nil and before is the item as read. before is the read the write was
// based on and after the item as stored.
func RewriteCommodity(
	ctx context.Context,
	reg registry.CommodityRegistry,
	id string,
	read *models.Commodity,
	change func(*models.Commodity) bool,
) (before, after *models.Commodity, err error) {
	current := read
	for attempt := 1; ; attempt++ {
		if current == nil {
			current, err = reg.Get(ctx, id)
			if err != nil {
				return nil, nil, err
			}
		}
		next := *current
		if !change(&next) {
			return current, nil, nil
		}
		after, err = reg.Update(registry.WithExpectedRevision(ctx, current.Revision), next)
		if err == nil {
			return current, after, nil
		}
		if !errors.Is(err, registry.ErrRevisionConflict) || attempt == commodityRewriteAttempts {
			return nil, nil, err
		}
		current = nil
	}
}
