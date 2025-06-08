package commonsql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ExportRegistry = (*ExportRegistry)(nil)

type ExportRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewExportRegistry(dbx *sqlx.DB) *ExportRegistry {
	return NewExportRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewExportRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *ExportRegistry {
	return &ExportRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *ExportRegistry) Create(ctx context.Context, export models.Export) (*models.Export, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Generate a new ID
	export.SetID(generateID())

	// Set created date if not set
	if export.CreatedDate == nil {
		export.CreatedDate = models.PNow()
	}

	// Insert the export
	err = InsertEntity(ctx, tx, r.tableNames.Exports(), export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert export")
	}

	return &export, nil
}

func (r *ExportRegistry) Get(ctx context.Context, id string) (*models.Export, error) {
	var export models.Export
	err := ScanEntityByField(ctx, r.dbx, r.tableNames.Exports(), "id", id, &export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get export")
	}

	return &export, nil
}

func (r *ExportRegistry) List(ctx context.Context) ([]*models.Export, error) {
	var exports []*models.Export

	// Query the database for non-deleted exports only (atomic operation)
	query := fmt.Sprintf("SELECT * FROM %s WHERE deleted_at IS NULL ORDER BY created_date DESC", r.tableNames.Exports())
	rows, err := r.dbx.QueryxContext(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to query exports")
	}
	defer rows.Close()

	for rows.Next() {
		var export models.Export
		if err := rows.StructScan(&export); err != nil {
			return nil, errkit.Wrap(err, "failed to scan export")
		}
		exports = append(exports, &export)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "failed to iterate exports")
	}

	return exports, nil
}

func (r *ExportRegistry) Update(ctx context.Context, export models.Export) (*models.Export, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Update the export
	err = UpdateEntityByField(ctx, tx, r.tableNames.Exports(), "id", export.ID, export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update export")
	}

	return &export, nil
}

func (r *ExportRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Soft delete the export by setting deleted_at timestamp
	deletedAt := string(models.Now())
	query := fmt.Sprintf("UPDATE %s SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL", r.tableNames.Exports())
	result, err := tx.ExecContext(ctx, query, deletedAt, id)
	if err != nil {
		return errkit.Wrap(err, "failed to soft delete export")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errkit.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errkit.Wrap(registry.ErrNotFound, "export not found or already deleted")
	}

	return nil
}

func (r *ExportRegistry) Count(ctx context.Context) (int, error) {
	// Count only non-deleted exports
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", r.tableNames.Exports())
	var count int
	err := r.dbx.GetContext(ctx, &count, query)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count exports")
	}

	return count, nil
}

// ListWithDeleted returns all exports including soft deleted ones
func (r *ExportRegistry) ListWithDeleted(ctx context.Context) ([]*models.Export, error) {
	var exports []*models.Export

	// Query the database for all exports including deleted ones (atomic operation)
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY created_date DESC", r.tableNames.Exports())
	rows, err := r.dbx.QueryxContext(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to query exports")
	}
	defer rows.Close()

	for rows.Next() {
		var export models.Export
		if err := rows.StructScan(&export); err != nil {
			return nil, errkit.Wrap(err, "failed to scan export")
		}
		exports = append(exports, &export)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "failed to iterate exports")
	}

	return exports, nil
}

// ListDeleted returns only soft deleted exports
func (r *ExportRegistry) ListDeleted(ctx context.Context) ([]*models.Export, error) {
	var exports []*models.Export

	// Query the database for deleted exports only (atomic operation)
	query := fmt.Sprintf("SELECT * FROM %s WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC", r.tableNames.Exports())
	rows, err := r.dbx.QueryxContext(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to query deleted exports")
	}
	defer rows.Close()

	for rows.Next() {
		var export models.Export
		if err := rows.StructScan(&export); err != nil {
			return nil, errkit.Wrap(err, "failed to scan export")
		}
		exports = append(exports, &export)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "failed to iterate deleted exports")
	}

	return exports, nil
}

// HardDelete permanently deletes an export from the database
func (r *ExportRegistry) HardDelete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Hard delete the export
	err = DeleteEntityByField(ctx, tx, r.tableNames.Exports(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to hard delete export")
	}

	return nil
}
