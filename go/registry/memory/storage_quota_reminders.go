package memory

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.StorageQuotaReminderRegistry = (*StorageQuotaReminderRegistry)(nil)

type baseStorageQuotaReminderRegistry = Registry[models.StorageQuotaReminder, *models.StorageQuotaReminder]

// StorageQuotaReminderRegistry is the in-memory implementation of the
// idempotency store for the storage quota warning worker (#1585).
// Mirrors the postgres registry — Create-once semantics enforced by
// the (group_id, threshold_percent) tuple; DeleteByGroupThreshold lets
// the worker reset the row when a group drops back below the
// threshold.
type StorageQuotaReminderRegistry struct {
	*baseStorageQuotaReminderRegistry
}

func NewStorageQuotaReminderRegistry() *StorageQuotaReminderRegistry {
	return &StorageQuotaReminderRegistry{
		baseStorageQuotaReminderRegistry: NewRegistry[models.StorageQuotaReminder, *models.StorageQuotaReminder](),
	}
}

// HasSent checks whether a row already exists for the given (group,
// threshold) tuple.
func (r *StorageQuotaReminderRegistry) HasSent(_ context.Context, groupID string, thresholdPercent int) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.GroupID == groupID && v.ThresholdPercent == thresholdPercent {
			return true, nil
		}
	}
	return false, nil
}

// CreateOnce inserts the reminder row iff no row exists for the same
// (group, threshold) tuple. Returns (false, nil) for the loser of a
// race so the caller can treat happy-path and race-loser identically.
//
// The check + insert run under a single lock acquisition — without
// that, two goroutines could both pass the existence scan, both
// proceed to Create, and end up with duplicate idempotency rows. We
// therefore insert directly into r.items here rather than delegating
// to baseStorageQuotaReminderRegistry.Create (which takes the lock
// again and would force us to drop ours mid-flight).
func (r *StorageQuotaReminderRegistry) CreateOnce(_ context.Context, reminder models.StorageQuotaReminder) (bool, error) {
	if reminder.SentAt.IsZero() {
		reminder.SentAt = time.Now()
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.GroupID == reminder.GroupID && v.ThresholdPercent == reminder.ThresholdPercent {
			return false, nil
		}
	}
	row := reminder
	row.ID = uuid.New().String()
	if row.UUID == "" {
		row.UUID = uuid.New().String()
	}
	r.items.Set(row.ID, &row)
	return true, nil
}

// DeleteByGroupThreshold removes the reminder row for the given
// (group, threshold) tuple. Returns true when a row was actually
// deleted — the worker uses that to log a "reset" event distinctly
// from the no-op case.
func (r *StorageQuotaReminderRegistry) DeleteByGroupThreshold(_ context.Context, groupID string, thresholdPercent int) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.GroupID == groupID && v.ThresholdPercent == thresholdPercent {
			r.items.Delete(pair.Key)
			return true, nil
		}
	}
	return false, nil
}
