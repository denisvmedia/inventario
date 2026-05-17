package memory

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
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
// (schedule, threshold) tuple. Input validation matches the Postgres
// registry (#1368 CR) — empty ScheduleID is a programmer error,
// invalid ThresholdDays would never match a real row and the worker
// must learn about the bug rather than silently miss reminders.
func (r *MaintenanceReminderRegistry) HasSent(_ context.Context, scheduleID string, thresholdDays int) (bool, error) {
	if scheduleID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ScheduleID"))
	}
	if !models.MaintenanceReminderThreshold(thresholdDays).IsValid() {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ThresholdDays"))
	}
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
// Mirrors the Postgres registry: validate the tuple up front and
// preserve caller-supplied IDs (only mint a fresh UUID when the
// incoming ID is empty).
func (r *MaintenanceReminderRegistry) CreateOnce(_ context.Context, reminder models.MaintenanceReminder) (bool, error) {
	if reminder.ScheduleID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ScheduleID"))
	}
	if !models.MaintenanceReminderThreshold(reminder.ThresholdDays).IsValid() {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ThresholdDays"))
	}
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
	// which we still hold. A caller-supplied ID is preserved (matches
	// the Postgres registry); only an empty ID gets a fresh UUID.
	row := reminder
	if row.ID == "" {
		row.ID = uuid.New().String()
	}
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
