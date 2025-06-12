package boltdb

import (
	"context"
	"errors"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameRestoreStep = "restore_step"
	bucketNameRestoreSteps = "restore-steps"
	idxRestoreStepsByOperation = "restore-steps-by-operation"
)

var _ registry.RestoreStepRegistry = (*RestoreStepRegistry)(nil)

type RestoreStepRegistry struct {
	db       *bolt.DB
	base     *dbx.BaseRepository[models.RestoreStep, *models.RestoreStep]
	registry *Registry[models.RestoreStep, *models.RestoreStep]
}

func NewRestoreStepRegistry(db *bolt.DB) *RestoreStepRegistry {
	base := dbx.NewBaseRepository[models.RestoreStep, *models.RestoreStep](bucketNameRestoreSteps)

	return &RestoreStepRegistry{
		db:   db,
		base: base,
		registry: NewRegistry[models.RestoreStep, *models.RestoreStep](
			db,
			base,
			entityNameRestoreStep,
			"", // No children bucket needed
		),
	}
}

func (r *RestoreStepRegistry) Create(ctx context.Context, step models.RestoreStep) (*models.RestoreStep, error) {
	result, err := r.registry.Create(step, func(tx dbx.TransactionOrBucket, step *models.RestoreStep) error {
		// Set timestamps
		step.CreatedDate = models.PNow()
		step.UpdatedDate = models.PNow()
		return nil
	}, func(tx dbx.TransactionOrBucket, step *models.RestoreStep) error {
		// Index by restore operation ID
		err := r.base.SaveIndexValue(tx, idxRestoreStepsByOperation, step.RestoreOperationID, step.ID)
		if err != nil {
			return errkit.Wrap(err, "failed to save restore step operation index")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create restore step")
	}

	return result, nil
}

func (r *RestoreStepRegistry) Get(_ context.Context, id string) (*models.RestoreStep, error) {
	return r.registry.Get(id)
}

func (r *RestoreStepRegistry) List(_ context.Context) ([]*models.RestoreStep, error) {
	return r.registry.List()
}

func (r *RestoreStepRegistry) Update(ctx context.Context, step models.RestoreStep) (*models.RestoreStep, error) {
	result, err := r.registry.Update(step, func(tx dbx.TransactionOrBucket, step *models.RestoreStep) error {
		// Update timestamp
		step.UpdatedDate = models.PNow()
		return nil
	}, func(tx dbx.TransactionOrBucket, step *models.RestoreStep) error {
		// Update index by restore operation ID
		err := r.base.SaveIndexValue(tx, idxRestoreStepsByOperation, step.RestoreOperationID, step.ID)
		if err != nil {
			return errkit.Wrap(err, "failed to update restore step operation index")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update restore step")
	}

	return result, nil
}

func (r *RestoreStepRegistry) Delete(ctx context.Context, id string) error {
	// Get the step first to remove from index
	step, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	return r.registry.Delete(id,
		func(tx dbx.TransactionOrBucket, s *models.RestoreStep) error {
			// Remove from index
			return r.base.DeleteIndexValue(tx, idxRestoreStepsByOperation, step.RestoreOperationID)
		},
		func(tx dbx.TransactionOrBucket, s *models.RestoreStep) error {
			return nil
		})
}

func (r *RestoreStepRegistry) Count(_ context.Context) (int, error) {
	return r.registry.Count()
}

func (r *RestoreStepRegistry) ListByRestoreOperation(_ context.Context, restoreOperationID string) ([]*models.RestoreStep, error) {
	var steps []*models.RestoreStep

	err := r.db.View(func(tx *bolt.Tx) error {
		// Get all steps and filter by restore operation ID
		allSteps, err := r.base.GetAll(tx, &models.RestoreStep{})
		if err != nil {
			if errors.Is(err, registry.ErrNotFound) {
				return nil // No steps found, return empty slice
			}
			return errkit.Wrap(err, "failed to get restore steps")
		}

		// Filter by restore operation ID
		for _, step := range allSteps {
			if step.RestoreOperationID == restoreOperationID {
				steps = append(steps, step)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return steps, nil
}

func (r *RestoreStepRegistry) DeleteByRestoreOperation(ctx context.Context, restoreOperationID string) error {
	steps, err := r.ListByRestoreOperation(ctx, restoreOperationID)
	if err != nil {
		return errkit.Wrap(err, "failed to list restore steps")
	}

	for _, step := range steps {
		if err := r.Delete(ctx, step.ID); err != nil {
			return errkit.Wrap(err, "failed to delete restore step")
		}
	}

	return nil
}
