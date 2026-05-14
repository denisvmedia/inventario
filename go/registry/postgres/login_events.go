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

var _ registry.LoginEventRegistry = (*LoginEventRegistry)(nil)

// LoginEventRegistry persists login_events. Like RefreshTokenRegistry,
// all access runs in service mode (inventario_background_worker role)
// — the write side is invoked from login flow where no user/tenant DB
// context exists yet, and the read side filters by user_id explicitly
// in application logic. See the row-level policy in models/login_event.go.
type LoginEventRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewLoginEventRegistry(dbx *sqlx.DB) *LoginEventRegistry {
	return NewLoginEventRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewLoginEventRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *LoginEventRegistry {
	return &LoginEventRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *LoginEventRegistry) newSQLRegistry() *store.RLSRepository[models.LoginEvent, *models.LoginEvent] {
	return store.NewServiceSQLRegistry[models.LoginEvent, *models.LoginEvent](r.dbx, r.tableNames.LoginEvents())
}

func (r *LoginEventRegistry) Create(ctx context.Context, event models.LoginEvent) (*models.LoginEvent, error) {
	if event.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if event.Email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}
	if event.Outcome == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Outcome"))
	}

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.UUID == "" {
		event.UUID = uuid.New().String()
	}
	if event.Method == "" {
		event.Method = models.LoginMethodPassword
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	reg := r.newSQLRegistry()
	if err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.LoginEvent](tx, r.tableNames.LoginEvents())
		return txReg.Insert(ctx, event)
	}); err != nil {
		return nil, errxtrace.Wrap("failed to insert login event", err)
	}
	return &event, nil
}

func (r *LoginEventRegistry) Get(ctx context.Context, id string) (*models.LoginEvent, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	var event models.LoginEvent
	reg := r.newSQLRegistry()
	if err := reg.ScanOneByField(ctx, store.Pair("id", id), &event); err != nil {
		return nil, errxtrace.Wrap("failed to get login event", err)
	}
	return &event, nil
}

func (r *LoginEventRegistry) List(ctx context.Context) ([]*models.LoginEvent, error) {
	var events []*models.LoginEvent
	reg := r.newSQLRegistry()
	for event, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list login events", err)
		}
		events = append(events, &event)
	}
	return events, nil
}

func (r *LoginEventRegistry) Update(_ context.Context, _ models.LoginEvent) (*models.LoginEvent, error) {
	// login_events is append-only by design — updates would corrupt the audit
	// trail. The Registry[T] interface requires Update to exist; treat
	// callers that hit it as a bug.
	return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("reason", "login_events is append-only"))
}

func (r *LoginEventRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	reg := r.newSQLRegistry()
	if err := reg.Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete login event", err)
	}
	return nil
}

func (r *LoginEventRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()
	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count login events", err)
	}
	return count, nil
}

func (r *LoginEventRegistry) ListByUser(ctx context.Context, userID string, limit int) ([]*models.LoginEvent, error) {
	if userID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	var events []*models.LoginEvent
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
			r.tableNames.LoginEvents(),
		)
		return tx.SelectContext(ctx, &events, query, userID, limit)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list login events by user", err)
	}
	return events, nil
}

func (r *LoginEventRegistry) CountFailedSince(ctx context.Context, userID string, since time.Time) (int, error) {
	if userID == "" {
		return 0, nil
	}
	var count int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE user_id = $1 AND outcome <> $2 AND created_at >= $3`,
			r.tableNames.LoginEvents(),
		)
		return tx.GetContext(ctx, &count, query, userID, models.LoginOutcomeOK, since)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count failed login events", err)
	}
	return count, nil
}

func (r *LoginEventRegistry) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error) {
	var deleted int64
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE created_at < $1`, r.tableNames.LoginEvents())
		res, err := tx.ExecContext(ctx, query, cutoff)
		if err != nil {
			return err
		}
		n, rerr := res.RowsAffected()
		if rerr == nil {
			deleted = n
		}
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to delete old login events", err)
	}
	return int(deleted), nil
}
