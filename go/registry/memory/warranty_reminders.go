package memory

import (
	"context"
	"time"

	"github.com/google/uuid"

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
// (commodity, threshold) tuple. Returns (false, nil) for the loser of
// a race so the caller can treat happy-path and race-loser identically.
//
// The check + insert run under a single lock acquisition — without
// that, two goroutines could both pass the existence scan, both
// proceed to Create, and end up with duplicate idempotency rows. We
// therefore insert directly into r.items here rather than delegating
// to baseWarrantyReminderRegistry.Create (which takes the lock again
// and would force us to drop ours mid-flight).
func (r *WarrantyReminderRegistry) CreateOnce(_ context.Context, reminder models.WarrantyReminder) (bool, error) {
	if reminder.SentAt.IsZero() {
		reminder.SentAt = time.Now()
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.CommodityID == reminder.CommodityID && v.ThresholdDays == reminder.ThresholdDays {
			return false, nil
		}
	}
	// Generate IDs ourselves — base.Create would re-acquire the lock,
	// which we still hold. The shape mirrors the base Create code path
	// (ID + UUID set server-side, never trusted from the caller).
	row := reminder
	row.ID = uuid.New().String()
	if row.UUID == "" {
		row.UUID = uuid.New().String()
	}
	r.items.Set(row.ID, &row)
	return true, nil
}
