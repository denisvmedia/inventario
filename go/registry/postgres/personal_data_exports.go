package postgres

import (
	"context"
	"database/sql"
	"errors"
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

var _ registry.PersonalDataExportRegistry = (*PersonalDataExportRegistry)(nil)

// personalDataExportLockKey namespaces the advisory lock that serializes one
// user's export requests.
const personalDataExportLockKey = "personal_data_export_request"

// PersonalDataExportRegistry stores personal data exports. Every query runs as
// inventario_background_worker (service mode): the export worker builds and
// expires every user's exports. Each statement that reads one user's rows
// names the user_id itself, so the wider role does not widen what a caller
// can reach.
type PersonalDataExportRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewPersonalDataExportRegistry returns a registry on the default table names.
func NewPersonalDataExportRegistry(dbx *sqlx.DB) *PersonalDataExportRegistry {
	return NewPersonalDataExportRegistryWithTableNames(dbx, store.DefaultTableNames)
}

// NewPersonalDataExportRegistryWithTableNames returns a registry on custom
// table names.
func NewPersonalDataExportRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *PersonalDataExportRegistry {
	return &PersonalDataExportRegistry{dbx: dbx, tableNames: tableNames}
}

func (r *PersonalDataExportRegistry) newSQLRegistry() *store.RLSRepository[models.PersonalDataExport, *models.PersonalDataExport] {
	return store.NewServiceSQLRegistry[models.PersonalDataExport, *models.PersonalDataExport](r.dbx, r.tableNames.PersonalDataExports())
}

func (r *PersonalDataExportRegistry) CreateUnlessRequestedSince(ctx context.Context, export models.PersonalDataExport, since time.Time) (created, blocking *models.PersonalDataExport, err error) {
	switch {
	case export.UserID == "":
		return nil, nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	case export.TenantID == "":
		return nil, nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	case export.ExpiresAt.IsZero():
		return nil, nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ExpiresAt"))
	}

	export.ID = uuid.New().String()
	export.UUID = uuid.New().String()
	if export.CreatedAt.IsZero() {
		export.CreatedAt = time.Now()
	}
	export.Status = models.PersonalDataExportStatusPending

	err = r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Two requests of one user read the same "no recent request" without
		// the lock and both insert. The lock is per user, so other users'
		// requests do not wait for it.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, personalDataExportLockKey, export.UserID); err != nil {
			return errxtrace.Wrap("failed to lock the user's export requests", err)
		}
		var latest models.PersonalDataExport
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE user_id = $1 AND created_at > $2 ORDER BY created_at DESC, id LIMIT 1`,
			r.tableNames.PersonalDataExports(),
		)
		switch err := tx.GetContext(ctx, &latest, query, export.UserID, since); {
		case err == nil:
			blocking = &latest
			return nil
		case !errors.Is(err, sql.ErrNoRows):
			return errxtrace.Wrap("failed to read the user's latest export request", err)
		}
		return store.NewTxRegistry[models.PersonalDataExport](tx, r.tableNames.PersonalDataExports()).Insert(ctx, export)
	})
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to create personal data export", err)
	}
	if blocking != nil {
		return nil, blocking, nil
	}
	return &export, nil, nil
}

func (r *PersonalDataExportRegistry) Get(ctx context.Context, id string) (*models.PersonalDataExport, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	return r.getOne(ctx, fmt.Sprintf(`SELECT * FROM %s WHERE id = $1`, r.tableNames.PersonalDataExports()), id)
}

func (r *PersonalDataExportRegistry) GetByUser(ctx context.Context, userID, id string) (*models.PersonalDataExport, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	return r.getOne(ctx, fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 AND user_id = $2`, r.tableNames.PersonalDataExports()), id, userID)
}

func (r *PersonalDataExportRegistry) getOne(ctx context.Context, query string, args ...any) (*models.PersonalDataExport, error) {
	var export models.PersonalDataExport
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &export, query, args...); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PersonalDataExport", "entity_id", args[0]))
			}
			return errxtrace.Wrap("failed to get personal data export", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &export, nil
}

func (r *PersonalDataExportRegistry) ListByUserID(ctx context.Context, userID string) ([]*models.PersonalDataExport, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	return r.list(ctx, fmt.Sprintf(
		`SELECT * FROM %s WHERE user_id = $1 ORDER BY created_at DESC, id`,
		r.tableNames.PersonalDataExports(),
	), userID)
}

func (r *PersonalDataExportRegistry) ListPending(ctx context.Context) ([]*models.PersonalDataExport, error) {
	return r.list(ctx, fmt.Sprintf(
		`SELECT * FROM %s WHERE status = $1 ORDER BY created_at, id`,
		r.tableNames.PersonalDataExports(),
	), string(models.PersonalDataExportStatusPending))
}

func (r *PersonalDataExportRegistry) ListExpired(ctx context.Context, now time.Time, limit int) ([]*models.PersonalDataExport, error) {
	if limit <= 0 {
		limit = 100
	}
	return r.list(ctx, fmt.Sprintf(
		`SELECT * FROM %s WHERE expires_at <= $1 ORDER BY expires_at, id LIMIT $2`,
		r.tableNames.PersonalDataExports(),
	), now, limit)
}

func (r *PersonalDataExportRegistry) list(ctx context.Context, query string, args ...any) ([]*models.PersonalDataExport, error) {
	exports := make([]*models.PersonalDataExport, 0)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &exports, query, args...)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list personal data exports", err)
	}
	return exports, nil
}

func (r *PersonalDataExportRegistry) ClaimPending(ctx context.Context, id string, startedAt time.Time) (bool, error) {
	if id == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	var claimed bool
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET status = $1, started_at = $2 WHERE id = $3 AND status = $4`,
			r.tableNames.PersonalDataExports())
		res, err := tx.ExecContext(ctx, query, string(models.PersonalDataExportStatusInProgress), startedAt,
			id, string(models.PersonalDataExportStatusPending))
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		claimed = n == 1
		return err
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to claim pending personal data export", err)
	}
	return claimed, nil
}

func (r *PersonalDataExportRegistry) RequeueStalled(ctx context.Context, startedBefore time.Time) (int, error) {
	var moved int64
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET status = $1, started_at = NULL WHERE status = $2 AND started_at < $3`,
			r.tableNames.PersonalDataExports())
		res, err := tx.ExecContext(ctx, query, string(models.PersonalDataExportStatusPending),
			string(models.PersonalDataExportStatusInProgress), startedBefore)
		if err != nil {
			return err
		}
		moved, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to queue stalled personal data exports again", err)
	}
	return int(moved), nil
}

func (r *PersonalDataExportRegistry) Update(ctx context.Context, export models.PersonalDataExport) (*models.PersonalDataExport, error) {
	if export.ID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if !export.Status.IsValid() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Status"))
	}
	var stored models.PersonalDataExport
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET
			status = $1, blob_key = $2, size_bytes = $3, group_count = $4, item_count = $5,
			file_count = $6, error_message = $7, completed_at = $8, expires_at = $9, started_at = $10
			WHERE id = $11 RETURNING *`, r.tableNames.PersonalDataExports())
		err := tx.GetContext(ctx, &stored, query,
			string(export.Status), export.BlobKey, export.SizeBytes, export.GroupCount, export.ItemCount,
			export.FileCount, export.ErrorMessage, export.CompletedAt, export.ExpiresAt, export.StartedAt, export.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PersonalDataExport", "entity_id", export.ID))
		}
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to update personal data export", err)
	}
	return &stored, nil
}

func (r *PersonalDataExportRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, r.tableNames.PersonalDataExports()), id)
		return err
	})
	if err != nil {
		return errxtrace.Wrap("failed to delete personal data export", err)
	}
	return nil
}
