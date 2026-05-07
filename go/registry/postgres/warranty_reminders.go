package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.WarrantyReminderRegistry = (*WarrantyReminderRegistry)(nil)

// WarrantyReminderRegistry is the postgres-backed idempotency store for
// the warranty reminder worker. Runs in service mode (background-worker
// role) so cross-tenant inserts during the periodic scan are not
// blocked by RLS.
type WarrantyReminderRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewWarrantyReminderRegistry(dbx *sqlx.DB) *WarrantyReminderRegistry {
	return &WarrantyReminderRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

func (r *WarrantyReminderRegistry) HasSent(ctx context.Context, commodityID string, thresholdDays int) (bool, error) {
	if commodityID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "CommodityID"))
	}
	var count int
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE commodity_id = $1 AND threshold_days = $2`,
			r.tableNames.WarrantyReminders(),
		)
		return tx.QueryRowxContext(ctx, query, commodityID, thresholdDays).Scan(&count)
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to check warranty reminder existence", err)
	}
	return count > 0, nil
}

// CreateOnce inserts the row iff no row exists for the same
// (commodity_id, threshold_days). Returns (false, nil) if the unique
// index already has the tuple — duplicate-key is a normal happy-path
// outcome here, not an error.
func (r *WarrantyReminderRegistry) CreateOnce(ctx context.Context, reminder models.WarrantyReminder) (bool, error) {
	if reminder.CommodityID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "CommodityID"))
	}
	if !models.WarrantyReminderThreshold(reminder.ThresholdDays).IsValid() {
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
			`INSERT INTO %s (id, tenant_id, group_id, created_by_user_id, commodity_id, threshold_days, sent_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (commodity_id, threshold_days) DO NOTHING`,
			r.tableNames.WarrantyReminders(),
		)
		res, execErr := tx.ExecContext(ctx, query,
			reminder.GetID(),
			reminder.TenantID,
			reminder.GroupID,
			reminder.CreatedByUserID,
			reminder.CommodityID,
			reminder.ThresholdDays,
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
		return false, errxtrace.Wrap("failed to insert warranty reminder", err)
	}
	return inserted, nil
}

// isUniqueViolation reports whether err corresponds to a Postgres
// 23505 (unique_violation) SQLSTATE. Pq embeds it via the Code field;
// we keep the check string-based to avoid pulling in lib/pq just for
// the constant — the SQLSTATE is a stable wire value across drivers.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	type sqlStater interface{ SQLState() string }
	var s sqlStater
	if errors.As(err, &s) {
		return s.SQLState() == "23505"
	}
	return strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}
