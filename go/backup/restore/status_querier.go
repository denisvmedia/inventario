package restore

import (
	"context"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// StatusQuerier reports aggregate status of restore operations without
// requiring a background worker goroutine to be running. It is intended for
// components that need to check whether a restore is currently in progress
// but do not themselves process restore work (e.g. an API-only process).
type StatusQuerier interface {
	// HasRunningRestores returns true if any restore operation is currently
	// running or pending.
	HasRunningRestores(ctx context.Context) (bool, error)
}

// RegistryStatusQuerier is a StatusQuerier backed solely by the restore
// operation registry. It performs no background processing and is safe to
// use in processes that do not run the RestoreWorker.
type RegistryStatusQuerier struct {
	registrySet *registry.Set
}

// NewRegistryStatusQuerier returns a RegistryStatusQuerier that reads restore
// operation state from the provided registry set.
func NewRegistryStatusQuerier(registrySet *registry.Set) *RegistryStatusQuerier {
	return &RegistryStatusQuerier{registrySet: registrySet}
}

// HasRunningRestores returns true if any restore operation in the registry is
// currently running or pending.
func (q *RegistryStatusQuerier) HasRunningRestores(ctx context.Context) (bool, error) {
	restoreOperations, err := q.registrySet.RestoreOperationRegistry.List(ctx)
	if err != nil {
		return false, err
	}
	for _, op := range restoreOperations {
		if op.Status == models.RestoreStatusRunning || op.Status == models.RestoreStatusPending {
			return true, nil
		}
	}
	return false, nil
}
