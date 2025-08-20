package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.RestoreOperationRegistry = (*RestoreOperationRegistry)(nil)

type RestoreOperationRegistry struct {
	db                  *sqlx.DB
	restoreStepRegistry registry.RestoreStepRegistry
}

func NewRestoreOperationRegistry(db *sqlx.DB, restoreStepRegistry registry.RestoreStepRegistry) *RestoreOperationRegistry {
	return &RestoreOperationRegistry{
		db:                  db,
		restoreStepRegistry: restoreStepRegistry,
	}
}

// SetUserContext sets the user context for RLS policies
func (r *RestoreOperationRegistry) SetUserContext(ctx context.Context, userID string) error {
	return SetUserContext(ctx, r.db, userID)
}

// WithUserContext executes a function with user context set
func (r *RestoreOperationRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	return WithUserContext(ctx, r.db, userID, fn)
}

func (r *RestoreOperationRegistry) Create(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	if err := operation.ValidateWithContext(ctx); err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Set timestamps
	operation.CreatedDate = models.PNow()

	// Generate ID if not set
	if operation.ID == "" {
		operation.ID = generateID()
	}

	// Set default status if not set
	if operation.Status == "" {
		operation.Status = models.RestoreStatusPending
	}

	// Serialize options to JSON
	optionsJSON, err := json.Marshal(operation.Options)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal options")
	}

	query := r.db.Rebind(`
		INSERT INTO restore_operations (
			id, export_id, description, status, options, created_date, started_date, 
			completed_date, error_message, location_count, area_count, commodity_count,
			image_count, invoice_count, manual_count, binary_data_size, error_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)

	_, err = r.db.ExecContext(ctx, query,
		operation.ID,
		operation.ExportID,
		operation.Description,
		operation.Status,
		string(optionsJSON),
		operation.CreatedDate,
		operation.StartedDate,
		operation.CompletedDate,
		operation.ErrorMessage,
		operation.LocationCount,
		operation.AreaCount,
		operation.CommodityCount,
		operation.ImageCount,
		operation.InvoiceCount,
		operation.ManualCount,
		operation.BinaryDataSize,
		operation.ErrorCount,
	)

	if err != nil {
		return nil, errkit.Wrap(err, "failed to create restore operation")
	}

	return &operation, nil
}

func (r *RestoreOperationRegistry) Get(ctx context.Context, id string) (*models.RestoreOperation, error) {
	query := r.db.Rebind(`
		SELECT id, export_id, description, status, options, created_date, started_date,
			   completed_date, error_message, location_count, area_count, commodity_count,
			   image_count, invoice_count, manual_count, binary_data_size, error_count
		FROM restore_operations 
		WHERE id = ?`)

	var operation models.RestoreOperation
	var optionsJSON string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&operation.ID,
		&operation.ExportID,
		&operation.Description,
		&operation.Status,
		&optionsJSON,
		&operation.CreatedDate,
		&operation.StartedDate,
		&operation.CompletedDate,
		&operation.ErrorMessage,
		&operation.LocationCount,
		&operation.AreaCount,
		&operation.CommodityCount,
		&operation.ImageCount,
		&operation.InvoiceCount,
		&operation.ManualCount,
		&operation.BinaryDataSize,
		&operation.ErrorCount,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errkit.Wrap(registry.ErrNotFound, "restore operation not found")
		}
		return nil, errkit.Wrap(err, "failed to get restore operation")
	}

	// Deserialize options from JSON
	if err := json.Unmarshal([]byte(optionsJSON), &operation.Options); err != nil {
		return nil, errkit.Wrap(err, "failed to unmarshal options")
	}

	// Load associated steps
	steps, err := r.restoreStepRegistry.ListByRestoreOperation(ctx, operation.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to load restore steps")
	}

	// Convert to slice of values instead of pointers for JSON serialization
	operation.Steps = make([]models.RestoreStep, len(steps))
	for i, step := range steps {
		operation.Steps[i] = *step
	}

	return &operation, nil
}

func (r *RestoreOperationRegistry) List(ctx context.Context) ([]*models.RestoreOperation, error) {
	query := r.db.Rebind(`
		SELECT id, export_id, description, status, options, created_date, started_date,
			   completed_date, error_message, location_count, area_count, commodity_count,
			   image_count, invoice_count, manual_count, binary_data_size, error_count
		FROM restore_operations 
		ORDER BY created_date DESC`)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to query restore operations")
	}
	defer rows.Close()

	var operations []*models.RestoreOperation
	for rows.Next() {
		var operation models.RestoreOperation
		var optionsJSON string
		err := rows.Scan(
			&operation.ID,
			&operation.ExportID,
			&operation.Description,
			&operation.Status,
			&optionsJSON,
			&operation.CreatedDate,
			&operation.StartedDate,
			&operation.CompletedDate,
			&operation.ErrorMessage,
			&operation.LocationCount,
			&operation.AreaCount,
			&operation.CommodityCount,
			&operation.ImageCount,
			&operation.InvoiceCount,
			&operation.ManualCount,
			&operation.BinaryDataSize,
			&operation.ErrorCount,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan restore operation")
		}

		// Deserialize options from JSON
		if err := json.Unmarshal([]byte(optionsJSON), &operation.Options); err != nil {
			return nil, errkit.Wrap(err, "failed to unmarshal options")
		}

		// Load associated steps
		steps, err := r.restoreStepRegistry.ListByRestoreOperation(ctx, operation.ID)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to load restore steps")
		}

		// Convert to slice of values instead of pointers for JSON serialization
		operation.Steps = make([]models.RestoreStep, len(steps))
		for i, step := range steps {
			operation.Steps[i] = *step
		}

		operations = append(operations, &operation)
	}

	if err = rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating restore operations")
	}

	return operations, nil
}

func (r *RestoreOperationRegistry) Update(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	if err := operation.ValidateWithContext(ctx); err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Serialize options to JSON
	optionsJSON, err := json.Marshal(operation.Options)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal options")
	}

	query := r.db.Rebind(`
		UPDATE restore_operations 
		SET description = ?, status = ?, options = ?, started_date = ?, completed_date = ?,
			error_message = ?, location_count = ?, area_count = ?, commodity_count = ?,
			image_count = ?, invoice_count = ?, manual_count = ?, binary_data_size = ?, error_count = ?
		WHERE id = ?`)

	result, err := r.db.ExecContext(ctx, query,
		operation.Description,
		operation.Status,
		string(optionsJSON),
		operation.StartedDate,
		operation.CompletedDate,
		operation.ErrorMessage,
		operation.LocationCount,
		operation.AreaCount,
		operation.CommodityCount,
		operation.ImageCount,
		operation.InvoiceCount,
		operation.ManualCount,
		operation.BinaryDataSize,
		operation.ErrorCount,
		operation.ID,
	)

	if err != nil {
		return nil, errkit.Wrap(err, "failed to update restore operation")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return nil, errkit.Wrap(registry.ErrNotFound, "restore operation not found")
	}

	return &operation, nil
}

func (r *RestoreOperationRegistry) Delete(ctx context.Context, id string) error {
	// Delete associated steps first (due to foreign key constraint)
	if err := r.restoreStepRegistry.DeleteByRestoreOperation(ctx, id); err != nil {
		return errkit.Wrap(err, "failed to delete restore steps")
	}

	query := r.db.Rebind(`DELETE FROM restore_operations WHERE id = ?`)

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete restore operation")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errkit.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errkit.Wrap(registry.ErrNotFound, "restore operation not found")
	}

	return nil
}

func (r *RestoreOperationRegistry) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM restore_operations`

	var count int
	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count restore operations")
	}

	return count, nil
}

func (r *RestoreOperationRegistry) ListByExport(ctx context.Context, exportID string) ([]*models.RestoreOperation, error) {
	query := r.db.Rebind(`
		SELECT id, export_id, description, status, options, created_date, started_date,
			   completed_date, error_message, location_count, area_count, commodity_count,
			   image_count, invoice_count, manual_count, binary_data_size, error_count
		FROM restore_operations 
		WHERE export_id = ?
		ORDER BY created_date DESC`)

	rows, err := r.db.QueryContext(ctx, query, exportID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to query restore operations by export")
	}
	defer rows.Close()

	var operations []*models.RestoreOperation
	for rows.Next() {
		var operation models.RestoreOperation
		var optionsJSON string
		err := rows.Scan(
			&operation.ID,
			&operation.ExportID,
			&operation.Description,
			&operation.Status,
			&optionsJSON,
			&operation.CreatedDate,
			&operation.StartedDate,
			&operation.CompletedDate,
			&operation.ErrorMessage,
			&operation.LocationCount,
			&operation.AreaCount,
			&operation.CommodityCount,
			&operation.ImageCount,
			&operation.InvoiceCount,
			&operation.ManualCount,
			&operation.BinaryDataSize,
			&operation.ErrorCount,
		)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to scan restore operation")
		}

		// Deserialize options from JSON
		if err := json.Unmarshal([]byte(optionsJSON), &operation.Options); err != nil {
			return nil, errkit.Wrap(err, "failed to unmarshal options")
		}

		// Load associated steps
		steps, err := r.restoreStepRegistry.ListByRestoreOperation(ctx, operation.ID)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to load restore steps")
		}

		// Convert to slice of values instead of pointers for JSON serialization
		operation.Steps = make([]models.RestoreStep, len(steps))
		for i, step := range steps {
			operation.Steps[i] = *step
		}

		operations = append(operations, &operation)
	}

	if err = rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating restore operations")
	}

	return operations, nil
}

// User-aware methods that automatically use user context from the request context

// CreateWithUser creates a restore operation with user context
func (r *RestoreOperationRegistry) CreateWithUser(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}
	operation.SetUserID(userID)
	return r.Create(ctx, operation)
}

// GetWithUser gets a restore operation with user context
func (r *RestoreOperationRegistry) GetWithUser(ctx context.Context, id string) (*models.RestoreOperation, error) {
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}
	err := SetUserContext(ctx, r.db, userID)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

// ListWithUser lists restore operations with user context
func (r *RestoreOperationRegistry) ListWithUser(ctx context.Context) ([]*models.RestoreOperation, error) {
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}
	err := SetUserContext(ctx, r.db, userID)
	if err != nil {
		return nil, err
	}
	return r.List(ctx)
}

// UpdateWithUser updates a restore operation with user context
func (r *RestoreOperationRegistry) UpdateWithUser(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}
	operation.SetUserID(userID)
	return r.Update(ctx, operation)
}

// DeleteWithUser deletes a restore operation with user context
func (r *RestoreOperationRegistry) DeleteWithUser(ctx context.Context, id string) error {
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}
	err := SetUserContext(ctx, r.db, userID)
	if err != nil {
		return err
	}
	return r.Delete(ctx, id)
}

// CountWithUser counts restore operations with user context
func (r *RestoreOperationRegistry) CountWithUser(ctx context.Context) (int, error) {
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return 0, errkit.WithStack(registry.ErrUserContextRequired)
	}
	err := SetUserContext(ctx, r.db, userID)
	if err != nil {
		return 0, err
	}
	return r.Count(ctx)
}
