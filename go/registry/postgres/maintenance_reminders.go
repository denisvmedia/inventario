package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.MaintenanceReminderRegistry = (*MaintenanceReminderRegistry)(nil)

// MaintenanceReminderRegistry is the postgres-backed idempotency store
// for the maintenance reminder worker (#1368). Runs in service mode
// (background-worker role) so cross-tenant inserts during the periodic
// scan are not blocked by RLS.
type MaintenanceReminderRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewMaintenanceReminderRegistry(dbx *sqlx.DB) *MaintenanceReminderRegistry {
	return &MaintenanceReminderRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

func (r *MaintenanceReminderRegistry) HasSent(ctx context.Context, scheduleID string, thresholdDays int) (bool, error) {
	if scheduleID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ScheduleID"))
	}
	var count int
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE schedule_id = $1 AND threshold_days = $2`,
			r.tableNames.MaintenanceReminders(),
		)
		return tx.QueryRowxContext(ctx, query, scheduleID, thresholdDays).Scan(&count)
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to check maintenance reminder existence", err)
	}
	return count > 0, nil
}

// CreateOnce inserts the row iff no row exists for the same
// (schedule_id, threshold_days). Returns (false, nil) if the unique
// index already has the tuple — duplicate-key is a normal happy-path
// outcome here, not an error.
func (r *MaintenanceReminderRegistry) CreateOnce(ctx context.Context, reminder models.MaintenanceReminder) (bool, error) {
	if reminder.ScheduleID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ScheduleID"))
	}
	if !models.MaintenanceReminderThreshold(reminder.ThresholdDays).IsValid() {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ThresholdDays"))
	}
	if reminder.SentAt.IsZero() {
		reminder.SentAt = time.Now()
	}
	if reminder.GetID() == "" {
		reminder.SetID(uuid.NewString())
	}

	inserted := false
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`INSERT INTO %s (id, tenant_id, group_id, created_by_user_id, schedule_id, threshold_days, sent_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (schedule_id, threshold_days) DO NOTHING`,
			r.tableNames.MaintenanceReminders(),
		)
		res, execErr := tx.ExecContext(ctx, query,
			reminder.GetID(),
			reminder.TenantID,
			reminder.GroupID,
			reminder.CreatedByUserID,
			reminder.ScheduleID,
			reminder.ThresholdDays,
			reminder.SentAt.UTC(),
		)
		if execErr != nil {
			// Treat unique-violation as the no-op outcome too — defence
			// in depth in case the conflict-target ever drifts from the
			// unique index.
			if isMaintenanceUniqueViolation(execErr) {
				return nil
			}
			return execErr
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			inserted = true
		}
		return nil
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to insert maintenance reminder", err)
	}
	return inserted, nil
}

// DeleteBySchedule removes every reminder row for the given schedule.
// Used by the service when the user marks a schedule done so the next
// cycle gets a clean idempotency state.
func (r *MaintenanceReminderRegistry) DeleteBySchedule(ctx context.Context, scheduleID string) (int, error) {
	if scheduleID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ScheduleID"))
	}
	var deleted int64
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE schedule_id = $1`, r.tableNames.MaintenanceReminders())
		res, execErr := tx.ExecContext(ctx, query, scheduleID)
		if execErr != nil {
			return execErr
		}
		deleted, _ = res.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to delete maintenance reminders by schedule", err)
	}
	return int(deleted), nil
}

// isMaintenanceUniqueViolation reports whether err corresponds to a
// Postgres 23505 (unique_violation) SQLSTATE. Kept locally so the
// maintenance registry stays self-contained.
func isMaintenanceUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	type sqlStater interface{ SQLState() string }
	var s sqlStater
	if errors.As(err, &s) {
		return s.SQLState() == "23505"
	}
	return false
}
