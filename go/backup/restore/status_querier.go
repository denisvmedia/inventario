package restore

import (
	"context"

	"go.5x5.cz/inventario/registry"
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
	return q.registrySet.RestoreOperationRegistry.HasActive(ctx)
}

// NoopStatusQuerier reports that nothing is running. It exists so a caller
// that does not care about the one-restore-at-a-time guard can still be wired
// with something, rather than with nil (#1314).
type NoopStatusQuerier struct{}

// HasRunningRestores always reports false.
func (NoopStatusQuerier) HasRunningRestores(context.Context) (bool, error) {
	return false, nil
}
