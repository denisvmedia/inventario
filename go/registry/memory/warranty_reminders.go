package memory

import (
	"context"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.WarrantyReminderRegistry = (*WarrantyReminderRegistry)(nil)

type baseWarrantyReminderRegistry = Registry[models.WarrantyReminder, *models.WarrantyReminder]

// WarrantyReminderRegistry is the in-memory implementation of the
// idempotency store. Mirrors the postgres registry — Create-once
// semantics enforced by the (commodity_id, threshold_days) tuple.
type WarrantyReminderRegistry struct {
	*baseWarrantyReminderRegistry
}

func NewWarrantyReminderRegistry() *WarrantyReminderRegistry {
	return &WarrantyReminderRegistry{
		baseWarrantyReminderRegistry: NewRegistry[models.WarrantyReminder, *models.WarrantyReminder](),
	}
}

// HasSent checks whether a row already exists for the given (commodity,
// threshold) tuple.
func (r *WarrantyReminderRegistry) HasSent(_ context.Context, commodityID string, thresholdDays int) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.CommodityID == commodityID && v.ThresholdDays == thresholdDays {
			return true, nil
		}
	}
	return false, nil
}

// CreateOnce inserts the reminder row iff no row exists for the same
// (commodity, threshold) tuple. Returns (false, nil) for the loser of a
// race so the caller can treat happy-path and race-loser identically.
func (r *WarrantyReminderRegistry) CreateOnce(ctx context.Context, reminder models.WarrantyReminder) (bool, error) {
	r.lock.Lock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.CommodityID == reminder.CommodityID && v.ThresholdDays == reminder.ThresholdDays {
			r.lock.Unlock()
			return false, nil
		}
	}
	r.lock.Unlock()
	if reminder.SentAt.IsZero() {
		reminder.SentAt = time.Now()
	}
	if _, err := r.baseWarrantyReminderRegistry.Create(ctx, reminder); err != nil {
		return false, err
	}
	return true, nil
}
