package postgres

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.RestoreStepRegistry = (*RestoreStepRegistry)(nil)

type RestoreStepRegistry struct {
	db *sqlx.DB
}

func NewRestoreStepRegistry(db *sqlx.DB) *RestoreStepRegistry {
	return &RestoreStepRegistry{db: db}
}

func (r *RestoreStepRegistry) Create(ctx context.Context, step models.RestoreStep) (*models.RestoreStep, error) {
	if err := step.ValidateWithContext(ctx); err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Set timestamps
	step.CreatedDate = models.PNow()
	step.UpdatedDate = models.PNow()

	// Generate ID if not set
	if step.ID == "" {
		step.ID = generateID()
	}

	query := r.db.Rebind(`
		INSERT INTO restore_steps (
			id, restore_operation_id, name, result, duration, reason, created_date, updated_date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)

	_, err := r.db.ExecContext(ctx, query,
		step.ID,
		step.RestoreOperationID,
		step.Name,
		step.Result,
		step.Duration,
		step.Reason,
		step.CreatedDate,
		step.UpdatedDate,
	)

	if err != nil {
		return nil, errkit.Wrap(err, "failed to create restore step")
	}

	return &step, nil
}

func (r *RestoreStepRegistry) Get(ctx context.Context, id string) (*models.RestoreStep, error) {
	query := r.db.Rebind(`
		SELECT id, restore_operation_id, name, result, duration, reason, created_date, updated_date
		FROM restore_steps
		WHERE id = ?`)

	var step models.RestoreStep
	err := r.db.GetContext(ctx, &step, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errkit.Wrap(registry.ErrNotFound, "restore step not found")
		}
		return nil, errkit.Wrap(err, "failed to get restore step")
	}

	return &step, nil
}

func (r *RestoreStepRegistry) List(ctx context.Context) ([]*models.RestoreStep, error) {
	query := `
		SELECT id, restore_operation_id, name, result, duration, reason, created_date, updated_date
		FROM restore_steps 
		ORDER BY created_date ASC`

	var steps []models.RestoreStep
	err := r.db.SelectContext(ctx, &steps, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list restore steps")
	}

	result := make([]*models.RestoreStep, len(steps))
	for i := range steps {
		result[i] = &steps[i]
	}

	return result, nil
}

func (r *RestoreStepRegistry) Update(ctx context.Context, step models.RestoreStep) (*models.RestoreStep, error) {
	if err := step.ValidateWithContext(ctx); err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Update timestamp
	step.UpdatedDate = models.PNow()

	query := r.db.Rebind(`
		UPDATE restore_steps
		SET name = ?, result = ?, duration = ?, reason = ?, updated_date = ?
		WHERE id = ?`)

	result, err := r.db.ExecContext(ctx, query,
		step.Name,
		step.Result,
		step.Duration,
		step.Reason,
		step.UpdatedDate,
		step.ID,
	)

	if err != nil {
		return nil, errkit.Wrap(err, "failed to update restore step")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return nil, errkit.Wrap(registry.ErrNotFound, "restore step not found")
	}

	return &step, nil
}

func (r *RestoreStepRegistry) Delete(ctx context.Context, id string) error {
	query := r.db.Rebind(`DELETE FROM restore_steps WHERE id = ?`)

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete restore step")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errkit.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errkit.Wrap(registry.ErrNotFound, "restore step not found")
	}

	return nil
}

func (r *RestoreStepRegistry) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM restore_steps`

	var count int
	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count restore steps")
	}

	return count, nil
}

func (r *RestoreStepRegistry) ListByRestoreOperation(ctx context.Context, restoreOperationID string) ([]*models.RestoreStep, error) {
	query := r.db.Rebind(`
		SELECT id, restore_operation_id, name, result, duration, reason, created_date, updated_date
		FROM restore_steps
		WHERE restore_operation_id = ?
		ORDER BY created_date ASC`)

	var steps []models.RestoreStep
	err := r.db.SelectContext(ctx, &steps, query, restoreOperationID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list restore steps by operation")
	}

	result := make([]*models.RestoreStep, len(steps))
	for i := range steps {
		result[i] = &steps[i]
	}

	return result, nil
}

func (r *RestoreStepRegistry) DeleteByRestoreOperation(ctx context.Context, restoreOperationID string) error {
	query := r.db.Rebind(`DELETE FROM restore_steps WHERE restore_operation_id = ?`)

	_, err := r.db.ExecContext(ctx, query, restoreOperationID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete restore steps by operation")
	}

	return nil
}
