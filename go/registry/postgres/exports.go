package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.ExportRegistry = (*ExportRegistry)(nil)

type ExportRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

func NewExportRegistry(dbx *sqlx.DB) *ExportRegistry {
	return NewExportRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewExportRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *ExportRegistry {
	return &ExportRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *ExportRegistry) MustWithCurrentUser(ctx context.Context) registry.ExportRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *ExportRegistry) WithCurrentUser(ctx context.Context) (registry.ExportRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}
	tmp.userID = user.ID
	tmp.tenantID = user.TenantID
	tmp.service = false
	return &tmp, nil
}

func (r *ExportRegistry) WithServiceAccount() registry.ExportRegistry {
	tmp := *r
	tmp.userID = ""
	tmp.tenantID = ""
	tmp.service = true
	return &tmp
}

func (r *ExportRegistry) Create(ctx context.Context, export models.Export) (*models.Export, error) {
	// Generate a new ID if one is not already provided
	if export.GetID() == "" {
		export.SetID(generateID())
	}

	export.SetTenantID(r.tenantID)
	export.SetUserID(r.userID)
	export.CreatedDate = models.PNow()

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, export, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create export")
	}

	return &export, nil
}

func (r *ExportRegistry) Get(ctx context.Context, id string) (*models.Export, error) {
	return r.get(ctx, id)
}

func (r *ExportRegistry) List(ctx context.Context) ([]*models.Export, error) {
	var exports []*models.Export

	reg := r.newSQLRegistry()

	// Query the database for non-deleted exports only
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE deleted_at IS NULL ORDER BY created_date DESC", r.tableNames.Exports())
		rows, err := tx.QueryxContext(ctx, query)
		if err != nil {
			return errkit.Wrap(err, "failed to query exports")
		}
		defer rows.Close()

		for rows.Next() {
			var export models.Export
			if err := rows.StructScan(&export); err != nil {
				return errkit.Wrap(err, "failed to scan export")
			}
			exports = append(exports, &export)
		}

		if err := rows.Err(); err != nil {
			return errkit.Wrap(err, "failed to iterate exports")
		}

		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list exports")
	}

	return exports, nil
}

func (r *ExportRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	var cnt int
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", r.tableNames.Exports())
		err := tx.GetContext(ctx, &cnt, query)
		if err != nil {
			return errkit.Wrap(err, "failed to count exports")
		}
		return nil
	})
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count exports")
	}

	return cnt, nil
}

func (r *ExportRegistry) Update(ctx context.Context, export models.Export) (*models.Export, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, export, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update export")
	}

	return &export, nil
}

func (r *ExportRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, func(ctx context.Context, tx *sqlx.Tx) error {
		// Get the export first to check if it has an associated file
		var export models.Export
		query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 AND deleted_at IS NULL", r.tableNames.Exports())
		err := tx.GetContext(ctx, &export, query, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errkit.Wrap(registry.ErrNotFound, "export not found or already deleted")
			}
			return errkit.Wrap(err, "failed to get export")
		}

		// Hard delete the export
		deleteExportQuery := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.tableNames.Exports())
		result, err := tx.ExecContext(ctx, deleteExportQuery, id)
		if err != nil {
			return errkit.Wrap(err, "failed to delete export")
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errkit.Wrap(err, "failed to get rows affected")
		}

		if rowsAffected == 0 {
			return errkit.Wrap(registry.ErrNotFound, "export not found")
		}

		return nil
	})

	return err
}

// ListWithDeleted returns all exports including soft deleted ones
func (r *ExportRegistry) ListWithDeleted(ctx context.Context) ([]*models.Export, error) {
	var exports []*models.Export

	reg := r.newSQLRegistry()

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT * FROM %s ORDER BY created_date DESC", r.tableNames.Exports())
		rows, err := tx.QueryxContext(ctx, query)
		if err != nil {
			return errkit.Wrap(err, "failed to query exports")
		}
		defer rows.Close()

		for rows.Next() {
			var export models.Export
			if err := rows.StructScan(&export); err != nil {
				return errkit.Wrap(err, "failed to scan export")
			}
			exports = append(exports, &export)
		}

		if err := rows.Err(); err != nil {
			return errkit.Wrap(err, "failed to iterate exports")
		}

		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list exports with deleted")
	}

	return exports, nil
}

// ListDeleted returns only soft deleted exports
func (r *ExportRegistry) ListDeleted(ctx context.Context) ([]*models.Export, error) {
	var exports []*models.Export

	reg := r.newSQLRegistry()

	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC", r.tableNames.Exports())
		rows, err := tx.QueryxContext(ctx, query)
		if err != nil {
			return errkit.Wrap(err, "failed to query deleted exports")
		}
		defer rows.Close()

		for rows.Next() {
			var export models.Export
			if err := rows.StructScan(&export); err != nil {
				return errkit.Wrap(err, "failed to scan export")
			}
			exports = append(exports, &export)
		}

		if err := rows.Err(); err != nil {
			return errkit.Wrap(err, "failed to iterate deleted exports")
		}

		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list deleted exports")
	}

	return exports, nil
}

// HardDelete permanently deletes an export from the database
func (r *ExportRegistry) HardDelete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txreg := store.NewTxRegistry[models.Export](tx, r.tableNames.Exports())
		err := txreg.DeleteByField(ctx, store.Pair("id", id))
		if err != nil {
			return errkit.Wrap(err, "failed to hard delete export")
		}
		return nil
	})

	return err
}

func (r *ExportRegistry) newSQLRegistry() *store.RLSRepository[models.Export, *models.Export] {
	if r.service {
		return store.NewServiceSQLRegistry[models.Export](r.dbx, r.tableNames.Exports())
	}
	return store.NewUserAwareSQLRegistry[models.Export](r.dbx, r.userID, r.tenantID, r.tableNames.Exports())
}

func (r *ExportRegistry) get(ctx context.Context, id string) (*models.Export, error) {
	var export models.Export
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get export")
	}

	return &export, nil
}
