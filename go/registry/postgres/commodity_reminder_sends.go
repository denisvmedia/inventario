package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

var _ registry.CommodityReminderSendRegistry = (*CommodityReminderSendRegistry)(nil)

// CommodityReminderSendRegistry is the postgres-backed idempotency store of
// the item reminder worker (#2805). It runs as the background-worker role so
// the cross-tenant sweep is not blocked by RLS.
type CommodityReminderSendRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewCommodityReminderSendRegistry(dbx *sqlx.DB) *CommodityReminderSendRegistry {
	return &CommodityReminderSendRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

func validateReminderSendKey(reminderID string, occurrenceOn models.Date) error {
	if reminderID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ReminderID"))
	}
	if occurrenceOn == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "OccurrenceOn"))
	}
	return nil
}

func (r *CommodityReminderSendRegistry) HasSentAtOrBelow(ctx context.Context, reminderID string, occurrenceOn models.Date, leadDays int) (bool, error) {
	if err := validateReminderSendKey(reminderID, occurrenceOn); err != nil {
		return false, err
	}
	var exists bool
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT EXISTS (SELECT 1 FROM %s WHERE reminder_id = $1 AND occurrence_on = $2 AND lead_days <= $3)`,
			r.tableNames.CommodityReminderSends(),
		)
		return tx.QueryRowxContext(ctx, query, reminderID, string(occurrenceOn), leadDays).Scan(&exists)
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to check commodity reminder send", err)
	}
	return exists, nil
}

// CreateOnce inserts the row unless one exists for the same (reminder,
// occurrence, lead time). A conflict is the expected outcome for the loser
// of a race between replicas and returns (false, nil).
func (r *CommodityReminderSendRegistry) CreateOnce(ctx context.Context, send models.CommodityReminderSend) (bool, error) {
	if err := validateReminderSendKey(send.ReminderID, send.OccurrenceOn); err != nil {
		return false, err
	}
	if send.SentAt.IsZero() {
		send.SentAt = time.Now()
	}
	if send.GetID() == "" {
		send.SetID(uuid.NewString())
	}
	if send.UUID == "" {
		send.UUID = uuid.NewString()
	}

	inserted := false
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`INSERT INTO %s (id, uuid, tenant_id, group_id, created_by_user_id, reminder_id, occurrence_on, lead_days, sent_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 ON CONFLICT (reminder_id, occurrence_on, lead_days) DO NOTHING`,
			r.tableNames.CommodityReminderSends(),
		)
		res, execErr := tx.ExecContext(ctx, query,
			send.GetID(),
			send.UUID,
			send.TenantID,
			send.GroupID,
			send.CreatedByUserID,
			send.ReminderID,
			string(send.OccurrenceOn),
			send.LeadDays,
			send.SentAt.UTC(),
		)
		if execErr != nil {
			if isMaintenanceUniqueViolation(execErr) {
				return nil
			}
			return execErr
		}
		n, _ := res.RowsAffected()
		inserted = n > 0
		return nil
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to insert commodity reminder send", err)
	}
	return inserted, nil
}

func (r *CommodityReminderSendRegistry) Release(ctx context.Context, reminderID string, occurrenceOn models.Date, leadDays int) error {
	if err := validateReminderSendKey(reminderID, occurrenceOn); err != nil {
		return err
	}
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`DELETE FROM %s WHERE reminder_id = $1 AND occurrence_on = $2 AND lead_days = $3`,
			r.tableNames.CommodityReminderSends(),
		)
		_, execErr := tx.ExecContext(ctx, query, reminderID, string(occurrenceOn), leadDays)
		return execErr
	})
	if err != nil {
		return errxtrace.Wrap("failed to release commodity reminder send", err)
	}
	return nil
}

func (r *CommodityReminderSendRegistry) DeleteByReminder(ctx context.Context, reminderID string) (int, error) {
	if reminderID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ReminderID"))
	}
	var deleted int64
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE reminder_id = $1`, r.tableNames.CommodityReminderSends())
		res, execErr := tx.ExecContext(ctx, query, reminderID)
		if execErr != nil {
			return execErr
		}
		deleted, _ = res.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to delete commodity reminder sends", err)
	}
	return int(deleted), nil
}
