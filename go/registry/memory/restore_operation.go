package memory

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// RestoreOperationRegistryFactory creates RestoreOperationRegistry instances with proper context
type RestoreOperationRegistryFactory struct {
	baseRestoreOperationRegistry *Registry[models.RestoreOperation, *models.RestoreOperation]
	restoreStepRegistry          *RestoreStepRegistryFactory
}

// RestoreOperationRegistry is a context-aware registry that can only be created through the factory
type RestoreOperationRegistry struct {
	*Registry[models.RestoreOperation, *models.RestoreOperation]
	restoreStepRegistry registry.RestoreStepRegistry

	userID string
}

var _ registry.RestoreOperationRegistry = (*RestoreOperationRegistry)(nil)
var _ registry.RestoreOperationRegistryFactory = (*RestoreOperationRegistryFactory)(nil)

func NewRestoreOperationRegistryFactory(restoreStepRegistry *RestoreStepRegistryFactory) *RestoreOperationRegistryFactory {
	return &RestoreOperationRegistryFactory{
		baseRestoreOperationRegistry: NewRegistry[models.RestoreOperation, *models.RestoreOperation](),
		restoreStepRegistry:          restoreStepRegistry,
	}
}

// Factory methods implementing registry.RestoreOperationRegistryFactory

func (f *RestoreOperationRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.RestoreOperationRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *RestoreOperationRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.RestoreOperationRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.RestoreOperation, *models.RestoreOperation]{
		items:  f.baseRestoreOperationRegistry.items, // Share the data map
		lock:   f.baseRestoreOperationRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                              // Set user-specific userID
	}

	// Create user-aware restore step registry
	restoreStepRegistry, err := f.restoreStepRegistry.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create user restore step registry")
	}

	return &RestoreOperationRegistry{
		Registry:            userRegistry,
		restoreStepRegistry: restoreStepRegistry,
		userID:              user.ID,
	}, nil
}

func (f *RestoreOperationRegistryFactory) CreateServiceRegistry() registry.RestoreOperationRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.RestoreOperation, *models.RestoreOperation]{
		items:  f.baseRestoreOperationRegistry.items, // Share the data map
		lock:   f.baseRestoreOperationRegistry.lock,  // Share the mutex pointer
		userID: "",                                   // Clear userID to bypass user filtering
	}

	// Create service-aware restore step registry
	restoreStepRegistry := f.restoreStepRegistry.CreateServiceRegistry()

	return &RestoreOperationRegistry{
		Registry:            serviceRegistry,
		restoreStepRegistry: restoreStepRegistry,
		userID:              "", // Clear userID to bypass user filtering
	}
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
