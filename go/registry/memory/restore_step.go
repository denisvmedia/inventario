package memory

import (
	"context"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.RestoreStepRegistry = (*RestoreStepRegistry)(nil)

type RestoreStepRegistry struct {
	*Registry[models.RestoreStep, *models.RestoreStep]
}

func NewRestoreStepRegistry() *RestoreStepRegistry {
	return &RestoreStepRegistry{
		Registry: NewRegistry[models.RestoreStep, *models.RestoreStep](),
	}
}

func (r *RestoreStepRegistry) ListByRestoreOperation(ctx context.Context, restoreOperationID string) ([]*models.RestoreStep, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var steps []*models.RestoreStep
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		step := pair.Value
		if step.RestoreOperationID == restoreOperationID {
			steps = append(steps, step)
		}
	}

	return steps, nil
}

func (r *RestoreStepRegistry) DeleteByRestoreOperation(ctx context.Context, restoreOperationID string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	var toDelete []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		step := pair.Value
		if step.RestoreOperationID == restoreOperationID {
			toDelete = append(toDelete, step.ID)
		}
	}

	for _, id := range toDelete {
		r.items.Delete(id)
	}

	return nil
}
