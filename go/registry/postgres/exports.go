package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// ExportRegistryFactory creates ExportRegistry instances with proper context
type ExportRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// ExportRegistry is a context-aware registry that can only be created through the factory
type ExportRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

var _ registry.ExportRegistry = (*ExportRegistry)(nil)
var _ registry.ExportRegistryFactory = (*ExportRegistryFactory)(nil)

func NewExportRegistry(dbx *sqlx.DB) *ExportRegistryFactory {
	return NewExportRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewExportRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *ExportRegistryFactory {
	return &ExportRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.ExportRegistryFactory

func (f *ExportRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ExportRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *ExportRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ExportRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user ID from context", err)
	}

	return &ExportRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *ExportRegistryFactory) CreateServiceRegistry() registry.ExportRegistry {
	return &ExportRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
		service:    true,
	}
}

func (r *ExportRegistry) Create(ctx context.Context, export models.Export) (*models.Export, error) {
	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create
	export.CreatedDate = models.PNow()

	reg := r.newSQLRegistry()

	createdExport, err := reg.Create(ctx, export, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create export", err)
	}

	return &createdExport, nil
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
			return errxtrace.Wrap("failed to query exports", err)
		}
		defer rows.Close()

		for rows.Next() {
			var export models.Export
			if err := rows.StructScan(&export); err != nil {
				return errxtrace.Wrap("failed to scan export", err)
			}
			exports = append(exports, &export)
		}

		if err := rows.Err(); err != nil {
			return errxtrace.Wrap("failed to iterate exports", err)
		}

		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list exports", err)
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
			return errxtrace.Wrap("failed to count exports", err)
		}
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count exports", err)
	}

	return cnt, nil
}

func (r *ExportRegistry) Update(ctx context.Context, export models.Export) (*models.Export, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, export, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update export", err)
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
				return errxtrace.Classify(registry.ErrNotFound, errx.NewDisplayable("export not found or already deleted"))
			}
			return errxtrace.Wrap("failed to get export", err)
		}

		// Hard delete the export
		deleteExportQuery := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.tableNames.Exports())
		result, err := tx.ExecContext(ctx, deleteExportQuery, id)
		if err != nil {
			return errxtrace.Wrap("failed to delete export", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return errxtrace.Wrap("failed to get rows affected", err)
		}

		if rowsAffected == 0 {
			return errxtrace.Wrap("export not found", registry.ErrNotFound)
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
			return errxtrace.Wrap("failed to query exports", err)
		}
		defer rows.Close()

		for rows.Next() {
			var export models.Export
			if err := rows.StructScan(&export); err != nil {
				return errxtrace.Wrap("failed to scan export", err)
			}
			exports = append(exports, &export)
		}

		if err := rows.Err(); err != nil {
			return errxtrace.Wrap("failed to iterate exports", err)
		}

		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list exports with deleted", err)
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
			return errxtrace.Wrap("failed to query deleted exports", err)
		}
		defer rows.Close()

		for rows.Next() {
			var export models.Export
			if err := rows.StructScan(&export); err != nil {
				return errxtrace.Wrap("failed to scan export", err)
			}
			exports = append(exports, &export)
		}

		if err := rows.Err(); err != nil {
			return errxtrace.Wrap("failed to iterate deleted exports", err)
		}

		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list deleted exports", err)
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
			return errxtrace.Wrap("failed to hard delete export", err)
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
		return nil, errxtrace.Wrap("failed to get export", err)
	}

	return &export, nil
}
