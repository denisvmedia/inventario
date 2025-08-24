package memory

import (
	"context"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.RestoreOperationRegistry = (*RestoreOperationRegistry)(nil)

type RestoreOperationRegistry struct {
	*Registry[models.RestoreOperation, *models.RestoreOperation]
	restoreStepRegistry registry.RestoreStepRegistry

	userID string
}

func NewRestoreOperationRegistry(restoreStepRegistry registry.RestoreStepRegistry) *RestoreOperationRegistry {
	return &RestoreOperationRegistry{
		Registry:            NewRegistry[models.RestoreOperation, *models.RestoreOperation](),
		restoreStepRegistry: restoreStepRegistry,
	}
}

func (r *RestoreOperationRegistry) WithCurrentUser(ctx context.Context) (registry.RestoreOperationRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}
	tmp.userID = user.ID
	return &tmp, nil
}

func (r *RestoreOperationRegistry) ListByExport(ctx context.Context, exportID string) ([]*models.RestoreOperation, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var operations []*models.RestoreOperation
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		operation := pair.Value
		if operation.ExportID == exportID {
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

			operations = append(operations, operation)
		}
	}

	return operations, nil
}

func (r *RestoreOperationRegistry) Get(ctx context.Context, id string) (*models.RestoreOperation, error) {
	operation, err := r.Registry.Get(ctx, id)
	if err != nil {
		return nil, err
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

	return operation, nil
}

func (r *RestoreOperationRegistry) Delete(ctx context.Context, id string) error {
	// Delete associated steps first
	if err := r.restoreStepRegistry.DeleteByRestoreOperation(ctx, id); err != nil {
		return errkit.Wrap(err, "failed to delete restore steps")
	}

	return r.Registry.Delete(ctx, id)
}
