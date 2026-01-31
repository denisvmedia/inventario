package memory

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// RestoreStepRegistryFactory creates RestoreStepRegistry instances with proper context
type RestoreStepRegistryFactory struct {
	baseRestoreStepRegistry *Registry[models.RestoreStep, *models.RestoreStep]
}

// RestoreStepRegistry is a context-aware registry that can only be created through the factory
type RestoreStepRegistry struct {
	*Registry[models.RestoreStep, *models.RestoreStep]

	userID string
}

var _ registry.RestoreStepRegistry = (*RestoreStepRegistry)(nil)
var _ registry.RestoreStepRegistryFactory = (*RestoreStepRegistryFactory)(nil)

func NewRestoreStepRegistryFactory() *RestoreStepRegistryFactory {
	return &RestoreStepRegistryFactory{
		baseRestoreStepRegistry: NewRegistry[models.RestoreStep, *models.RestoreStep](),
	}
}

// Factory methods implementing registry.RestoreStepRegistryFactory

func (f *RestoreStepRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.RestoreStepRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *RestoreStepRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.RestoreStepRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.RestoreStep, *models.RestoreStep]{
		items:  f.baseRestoreStepRegistry.items, // Share the data map
		lock:   f.baseRestoreStepRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                         // Set user-specific userID
	}

	return &RestoreStepRegistry{
		Registry: userRegistry,
		userID:   user.ID,
	}, nil
}

func (f *RestoreStepRegistryFactory) CreateServiceRegistry() registry.RestoreStepRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.RestoreStep, *models.RestoreStep]{
		items:  f.baseRestoreStepRegistry.items, // Share the data map
		lock:   f.baseRestoreStepRegistry.lock,  // Share the mutex pointer
		userID: "",                              // Clear userID to bypass user filtering
	}

	return &RestoreStepRegistry{
		Registry: serviceRegistry,
		userID:   "", // Clear userID to bypass user filtering
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
