package postgres

import (
	"context"
	"database/sql"
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

// SetUserContext sets the user context for RLS policies
func (r *ExportRegistry) SetUserContext(ctx context.Context, userID string) error {
	return SetUserContext(ctx, r.dbx, userID)
}

// WithUserContext executes a function with user context set
func (r *ExportRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	return WithUserContext(ctx, r.dbx, userID, fn)
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

	// Get the export first to check if it has an associated file
	var export models.Export
	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1 AND deleted_at IS NULL", r.tableNames.Exports())
	err = tx.GetContext(ctx, &export, query, id)
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

// User-aware methods that automatically use user context from the request context

// CreateWithUser creates an export with user context
func (r *ExportRegistry) CreateWithUser(ctx context.Context, export models.Export) (*models.Export, error) {
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}
	export.SetUserID(userID)
	if export.GetID() == "" {
		export.SetID(generateID())
	}
	err := InsertEntityWithUser(ctx, r.dbx, r.tableNames.Exports(), export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}
	return &export, nil
}

// GetWithUser gets an export with user context
func (r *ExportRegistry) GetWithUser(ctx context.Context, id string) (*models.Export, error) {
	var export models.Export
	err := ScanEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Exports(), "id", id, &export)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, errkit.WithStack(registry.ErrNotFound, "entity_type", "Export", "entity_id", id)
		}
		return nil, errkit.Wrap(err, "failed to get entity")
	}
	return &export, nil
}

// ListWithUser lists exports with user context
func (r *ExportRegistry) ListWithUser(ctx context.Context) ([]*models.Export, error) {
	var exports []*models.Export
	for export, err := range ScanEntitiesWithUser[models.Export](ctx, r.dbx, r.tableNames.Exports()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list exports")
		}
		exports = append(exports, &export)
	}
	return exports, nil
}

// UpdateWithUser updates an export with user context
func (r *ExportRegistry) UpdateWithUser(ctx context.Context, export models.Export) (*models.Export, error) {
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}
	export.SetUserID(userID)
	err := UpdateEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Exports(), "id", export.ID, export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update entity")
	}
	return &export, nil
}

// DeleteWithUser deletes an export with user context
func (r *ExportRegistry) DeleteWithUser(ctx context.Context, id string) error {
	return DeleteEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Exports(), "id", id)
}

// CountWithUser counts exports with user context
func (r *ExportRegistry) CountWithUser(ctx context.Context) (int, error) {
	return CountEntitiesWithUser(ctx, r.dbx, r.tableNames.Exports())
}
