package memory

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.MaintenanceReminderRegistry = (*MaintenanceReminderRegistry)(nil)

type baseMaintenanceReminderRegistry = Registry[models.MaintenanceReminder, *models.MaintenanceReminder]

// MaintenanceReminderRegistry is the in-memory implementation of the
// maintenance reminder idempotency store. Mirrors the warranty reminder
// in-memory registry — Create-once semantics enforced by the
// (schedule_id, threshold_days) tuple.
type MaintenanceReminderRegistry struct {
	*baseMaintenanceReminderRegistry
}

func NewMaintenanceReminderRegistry() *MaintenanceReminderRegistry {
	return &MaintenanceReminderRegistry{
		baseMaintenanceReminderRegistry: NewRegistry[models.MaintenanceReminder, *models.MaintenanceReminder](),
	}
}

// HasSent checks whether a row already exists for the given
// (schedule, threshold) tuple.
func (r *MaintenanceReminderRegistry) HasSent(_ context.Context, scheduleID string, thresholdDays int) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.ScheduleID == scheduleID && v.ThresholdDays == thresholdDays {
			return true, nil
		}
	}
	return false, nil
}

// CreateOnce inserts the reminder row iff no row exists for the same
// (schedule, threshold) tuple. Returns (false, nil) for the loser of
// a race so the caller can treat happy-path and race-loser identically.
func (r *MaintenanceReminderRegistry) CreateOnce(_ context.Context, reminder models.MaintenanceReminder) (bool, error) {
	if reminder.SentAt.IsZero() {
		reminder.SentAt = time.Now()
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.ScheduleID == reminder.ScheduleID && v.ThresholdDays == reminder.ThresholdDays {
			return false, nil
		}
	}
	// Generate IDs ourselves — base.Create would re-acquire the lock,
	// which we still hold.
	row := reminder
	row.ID = uuid.New().String()
	if row.UUID == "" {
		row.UUID = uuid.New().String()
	}
	r.items.Set(row.ID, &row)
	return true, nil
}

// DeleteBySchedule removes every reminder row for the given schedule.
// Returns the number of rows deleted.
func (r *MaintenanceReminderRegistry) DeleteBySchedule(_ context.Context, scheduleID string) (int, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var victims []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.ScheduleID == scheduleID {
			victims = append(victims, v.ID)
		}
	}
	for _, id := range victims {
		r.items.Delete(id)
	}
	return len(victims), nil
}
