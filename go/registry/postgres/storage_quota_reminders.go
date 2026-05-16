package postgres

import (
	"context"
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

var _ registry.StorageQuotaReminderRegistry = (*StorageQuotaReminderRegistry)(nil)

// StorageQuotaReminderRegistry is the postgres-backed idempotency
// store for the storage quota warning worker (#1585). Runs in service
// mode (background-worker role) so cross-tenant inserts during the
// periodic scan are not blocked by RLS.
type StorageQuotaReminderRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewStorageQuotaReminderRegistry(dbx *sqlx.DB) *StorageQuotaReminderRegistry {
	return &StorageQuotaReminderRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

// HasSent reports whether a row already exists for the given (group,
// threshold) tuple.
func (r *StorageQuotaReminderRegistry) HasSent(ctx context.Context, groupID string, thresholdPercent int) (bool, error) {
	if groupID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	var count int
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE group_id = $1 AND threshold_percent = $2`,
			r.tableNames.StorageQuotaReminders(),
		)
		return tx.QueryRowxContext(ctx, query, groupID, thresholdPercent).Scan(&count)
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to check storage quota reminder existence", err)
	}
	return count > 0, nil
}

// CreateOnce inserts the row iff no row exists for the same
// (group_id, threshold_percent). Returns (false, nil) if the unique
// index already has the tuple — duplicate-key is a normal happy-path
// outcome here, not an error.
func (r *StorageQuotaReminderRegistry) CreateOnce(ctx context.Context, reminder models.StorageQuotaReminder) (bool, error) {
	if reminder.GroupID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	if !models.StorageQuotaThreshold(reminder.ThresholdPercent).IsValid() {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ThresholdPercent"))
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
			`INSERT INTO %s (id, tenant_id, group_id, created_by_user_id, threshold_percent, sent_at)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (group_id, threshold_percent) DO NOTHING`,
			r.tableNames.StorageQuotaReminders(),
		)
		res, execErr := tx.ExecContext(ctx, query,
			reminder.GetID(),
			reminder.TenantID,
			reminder.GroupID,
			reminder.CreatedByUserID,
			reminder.ThresholdPercent,
			reminder.SentAt.UTC(),
		)
		if execErr != nil {
			// Treat unique-violation as the no-op outcome too — defence
			// in depth in case the conflict-target ever drifts from the
			// unique index.
			if isUniqueViolation(execErr) {
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
		return false, errxtrace.Wrap("failed to insert storage quota reminder", err)
	}
	return inserted, nil
}

// DeleteByGroupThreshold removes the reminder row for the given
// (group, threshold) tuple. Returns true when a row was actually
// deleted so the caller can log a "reset" event distinctly from the
// no-op case. Used by the worker when a group drops back below the
// threshold so a future re-crossing fires a fresh email.
func (r *StorageQuotaReminderRegistry) DeleteByGroupThreshold(ctx context.Context, groupID string, thresholdPercent int) (bool, error) {
	if groupID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	deleted := false
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`DELETE FROM %s WHERE group_id = $1 AND threshold_percent = $2`,
			r.tableNames.StorageQuotaReminders(),
		)
		res, execErr := tx.ExecContext(ctx, query, groupID, thresholdPercent)
		if execErr != nil {
			return execErr
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			deleted = true
		}
		return nil
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to delete storage quota reminder", err)
	}
	return deleted, nil
}
