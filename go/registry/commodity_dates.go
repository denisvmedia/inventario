package registry

import (
	"context"
	"time"

	"go.5x5.cz/inventario/models"
)

// keptModifiedDateCtxKey keys the signal that a commodity write keeps the
// caller's last-modified date (see WithKeptModifiedDate).
type keptModifiedDateCtxKey struct{}

// WithKeptModifiedDate marks ctx so CommodityRegistry.Update stores the
// LastModifiedDate the caller gives instead of today. A backup restore sets
// it, so a restored item keeps the archived date through the writes that
// follow its creation (the cover photo, the item links, a merge into an
// existing item). A write that only follows a change elsewhere, such as the
// memory registry's tag rename, sets it too and passes the item's own date.
// An empty date is still stamped with today. The API path never sets it.
func WithKeptModifiedDate(ctx context.Context) context.Context {
	return context.WithValue(ctx, keptModifiedDateCtxKey{}, true)
}

// keptModifiedDate reports whether ctx carries WithKeptModifiedDate.
func keptModifiedDate(ctx context.Context) bool {
	kept, _ := ctx.Value(keptModifiedDateCtxKey{}).(bool)
	return kept
}

// StampNewCommodity sets the dates of an item being created: the day it was
// added (RegisteredDate) and the day it was last changed (LastModifiedDate)
// are today, in UTC (see models.NewDate), unless the caller gives them. A
// backup restore or an API client that carries a date keeps it.
func StampNewCommodity(commodity *models.Commodity, now time.Time) {
	today := models.NewDate(now)
	if commodity.RegisteredDate.IsEmpty() {
		commodity.RegisteredDate = new(today)
	}
	if commodity.LastModifiedDate.IsEmpty() {
		commodity.LastModifiedDate = new(today)
	}
}

// StampEditedCommodity sets the dates of an item being edited. The day it
// was added is the caller's when given and stored, the item's current one,
// otherwise: an edit form does not send it back. The day it was last changed
// is today, in UTC; under WithKeptModifiedDate a date the caller gives is
// kept instead.
func StampEditedCommodity(ctx context.Context, commodity *models.Commodity, stored models.PDate, now time.Time) {
	if commodity.RegisteredDate.IsEmpty() {
		commodity.RegisteredDate = nil
		if !stored.IsEmpty() {
			commodity.RegisteredDate = new(*stored)
		}
	}
	if keptModifiedDate(ctx) && !commodity.LastModifiedDate.IsEmpty() {
		return
	}
	commodity.LastModifiedDate = new(models.NewDate(now))
}
