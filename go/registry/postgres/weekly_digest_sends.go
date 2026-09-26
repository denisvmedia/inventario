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

var _ registry.WeeklyDigestSendRegistry = (*WeeklyDigestSendRegistry)(nil)

// WeeklyDigestSendRegistry is the postgres-backed idempotency store for the
// weekly digest worker (#1391). It runs as the background-worker role: the
// sweep crosses every tenant, and no user context exists on the connection.
type WeeklyDigestSendRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewWeeklyDigestSendRegistry(dbx *sqlx.DB) *WeeklyDigestSendRegistry {
	return &WeeklyDigestSendRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

// ClaimWeek inserts the row that marks this user's week as sent, and reports
// whether this call is the one that inserted it.
//
// The insert is the claim rather than a check followed by a write: two workers
// racing on the same week both see no row, and only the unique index can decide
// between them. A losing insert is an ordinary outcome, not an error.
func (r *WeeklyDigestSendRegistry) ClaimWeek(ctx context.Context, send models.WeeklyDigestSend) (bool, error) {
	if send.TenantID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if send.UserID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if send.WeekStart.IsZero() {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "WeekStart"))
	}
	if send.SentAt.IsZero() {
		send.SentAt = time.Now()
	}
	if send.GetID() == "" {
		send.SetID(uuid.NewString())
	}

	claimed := false
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`INSERT INTO %s (id, tenant_id, user_id, week_start, sent_at)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (user_id, week_start) DO NOTHING`,
			r.tableNames.WeeklyDigestSends(),
		)
		res, execErr := tx.ExecContext(ctx, query,
			send.GetID(),
			send.TenantID,
			send.UserID,
			send.WeekStart.UTC(),
			send.SentAt.UTC(),
		)
		if execErr != nil {
			// Defence in depth, in case the conflict target ever drifts from
			// the unique index: a duplicate is still the no-op outcome.
			if isUniqueViolation(execErr) {
				return nil
			}
			return execErr
		}
		n, _ := res.RowsAffected()
		claimed = n > 0
		return nil
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to claim weekly digest week", err)
	}
	return claimed, nil
}

// ReleaseWeek drops one user's claim on a week.
func (r *WeeklyDigestSendRegistry) ReleaseWeek(ctx context.Context, userID string, weekStart time.Time) error {
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if weekStart.IsZero() {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "WeekStart"))
	}
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`DELETE FROM %s WHERE user_id = $1 AND week_start = $2`,
			r.tableNames.WeeklyDigestSends(),
		)
		_, execErr := tx.ExecContext(ctx, query, userID, weekStart.UTC())
		return execErr
	})
	if err != nil {
		return errxtrace.Wrap("failed to release weekly digest week", err)
	}
	return nil
}

// DeleteSentBefore removes claims for weeks starting before the given date and
// returns how many went. The rows exist to stop a second send inside one week,
// so anything older than a few weeks is dead weight.
func (r *WeeklyDigestSendRegistry) DeleteSentBefore(ctx context.Context, weekStart time.Time) (int, error) {
	if weekStart.IsZero() {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "WeekStart"))
	}
	deleted := 0
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`DELETE FROM %s WHERE week_start < $1`,
			r.tableNames.WeeklyDigestSends(),
		)
		res, execErr := tx.ExecContext(ctx, query, weekStart.UTC())
		if execErr != nil {
			return execErr
		}
		n, _ := res.RowsAffected()
		deleted = int(n)
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to delete old weekly digest claims", err)
	}
	return deleted, nil
}
