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
	entityNameRestoreOperation   = "restore_operation"
	bucketNameRestoreOperations  = "restore-operations"
	idxRestoreOperationsByExport = "restore-operations-by-export"
)

var _ registry.RestoreOperationRegistry = (*RestoreOperationRegistry)(nil)

type RestoreOperationRegistry struct {
	db                  *bolt.DB
	base                *dbx.BaseRepository[models.RestoreOperation, *models.RestoreOperation]
	registry            *Registry[models.RestoreOperation, *models.RestoreOperation]
	restoreStepRegistry registry.RestoreStepRegistry
}

func NewRestoreOperationRegistry(db *bolt.DB, restoreStepRegistry registry.RestoreStepRegistry) *RestoreOperationRegistry {
	base := dbx.NewBaseRepository[models.RestoreOperation, *models.RestoreOperation](bucketNameRestoreOperations)

	return &RestoreOperationRegistry{
		db:   db,
		base: base,
		registry: NewRegistry[models.RestoreOperation, *models.RestoreOperation](
			db,
			base,
			entityNameRestoreOperation,
			"", // No children bucket needed
		),
		restoreStepRegistry: restoreStepRegistry,
	}
}

func (r *RestoreOperationRegistry) Create(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	result, err := r.registry.Create(operation, func(tx dbx.TransactionOrBucket, operation *models.RestoreOperation) error {
		// Set timestamps
		operation.CreatedDate = models.PNow()

		// Set default status if not set
		if operation.Status == "" {
			operation.Status = models.RestoreStatusPending
		}

		return nil
	}, func(tx dbx.TransactionOrBucket, operation *models.RestoreOperation) error {
		// Index by export ID
		err := r.base.SaveIndexValue(tx, idxRestoreOperationsByExport, operation.ExportID, operation.ID)
		if err != nil {
			return errkit.Wrap(err, "failed to save restore operation export index")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create restore operation")
	}

	return result, nil
}

func (r *RestoreOperationRegistry) Get(ctx context.Context, id string) (*models.RestoreOperation, error) {
	operation, err := r.registry.Get(id)
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

func (r *RestoreOperationRegistry) List(ctx context.Context) ([]*models.RestoreOperation, error) {
	operations, err := r.registry.List()
	if err != nil {
		return nil, err
	}

	// Load steps for each operation
	for _, operation := range operations {
		steps, err := r.restoreStepRegistry.ListByRestoreOperation(ctx, operation.ID)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to load restore steps")
		}

		// Convert to slice of values instead of pointers for JSON serialization
		operation.Steps = make([]models.RestoreStep, len(steps))
		for i, step := range steps {
			operation.Steps[i] = *step
		}
	}

	return operations, nil
}

func (r *RestoreOperationRegistry) Update(ctx context.Context, operation models.RestoreOperation) (*models.RestoreOperation, error) {
	result, err := r.registry.Update(operation, nil, func(tx dbx.TransactionOrBucket, operation *models.RestoreOperation) error {
		// Update index by export ID
		err := r.base.SaveIndexValue(tx, idxRestoreOperationsByExport, operation.ExportID, operation.ID)
		if err != nil {
			return errkit.Wrap(err, "failed to update restore operation export index")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update restore operation")
	}

	return result, nil
}

func (r *RestoreOperationRegistry) Delete(ctx context.Context, id string) error {
	// Get the operation first to remove from index
	operation, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	// Delete associated steps first
	if err := r.restoreStepRegistry.DeleteByRestoreOperation(ctx, id); err != nil {
		return errkit.Wrap(err, "failed to delete restore steps")
	}

	return r.registry.Delete(id,
		func(tx dbx.TransactionOrBucket, op *models.RestoreOperation) error {
			// Remove from index
			return r.base.DeleteIndexValue(tx, idxRestoreOperationsByExport, operation.ExportID)
		},
		func(tx dbx.TransactionOrBucket, op *models.RestoreOperation) error {
			return nil
		})
}

func (r *RestoreOperationRegistry) Count(_ context.Context) (int, error) {
	return r.registry.Count()
}

func (r *RestoreOperationRegistry) ListByExport(ctx context.Context, exportID string) ([]*models.RestoreOperation, error) {
	var operations []*models.RestoreOperation

	err := r.db.View(func(tx *bolt.Tx) error {
		// Get all operations and filter by export ID
		allOperations, err := r.base.GetAll(tx, &models.RestoreOperation{})
		if err != nil {
			if errors.Is(err, registry.ErrNotFound) {
				return nil // No operations found, return empty slice
			}
			return errkit.Wrap(err, "failed to get restore operations")
		}

		// Filter by export ID
		for _, operation := range allOperations {
			if operation.ExportID == exportID {
				operations = append(operations, operation)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Load steps for each operation
	for _, operation := range operations {
		steps, err := r.restoreStepRegistry.ListByRestoreOperation(ctx, operation.ID)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to load restore steps")
		}

		// Convert to slice of values instead of pointers for JSON serialization
		operation.Steps = make([]models.RestoreStep, len(steps))
		for i, step := range steps {
			operation.Steps[i] = *step
		}
	}

	return operations, nil
}
