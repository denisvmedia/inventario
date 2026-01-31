package memory

import (
	"context"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ExportRegistryFactory creates ExportRegistry instances with proper context
type ExportRegistryFactory struct {
	baseExportRegistry *Registry[models.Export, *models.Export]
}

// ExportRegistry is a context-aware registry that can only be created through the factory
type ExportRegistry struct {
	*Registry[models.Export, *models.Export]

	userID string
}

var _ registry.ExportRegistry = (*ExportRegistry)(nil)
var _ registry.ExportRegistryFactory = (*ExportRegistryFactory)(nil)

func NewExportRegistryFactory() *ExportRegistryFactory {
	return &ExportRegistryFactory{
		baseExportRegistry: NewRegistry[models.Export, *models.Export](),
	}
}

// Factory methods implementing registry.ExportRegistryFactory

func (f *ExportRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ExportRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *ExportRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ExportRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.Export, *models.Export]{
		items:  f.baseExportRegistry.items, // Share the data map
		lock:   f.baseExportRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                    // Set user-specific userID
	}

	return &ExportRegistry{
		Registry: userRegistry,
		userID:   user.ID,
	}, nil
}

func (f *ExportRegistryFactory) CreateServiceRegistry() registry.ExportRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.Export, *models.Export]{
		items:  f.baseExportRegistry.items, // Share the data map
		lock:   f.baseExportRegistry.lock,  // Share the mutex pointer
		userID: "",                         // Clear userID to bypass user filtering
	}

	return &ExportRegistry{
		Registry: serviceRegistry,
		userID:   "", // Clear userID to bypass user filtering
	}
}

// List returns only non-deleted exports
func (r *ExportRegistry) List(ctx context.Context) ([]*models.Export, error) {
	allExports, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}

	// Filter out deleted exports
	var activeExports []*models.Export
	for _, export := range allExports {
		if !export.IsDeleted() {
			activeExports = append(activeExports, export)
		}
	}

	return activeExports, nil
}

// Get returns an export by ID, excluding soft deleted exports
func (r *ExportRegistry) Get(ctx context.Context, id string) (*models.Export, error) {
	export, err := r.Registry.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if export.IsDeleted() {
		return export, errxtrace.Classify(registry.ErrDeleted, errx.Attrs("reason", "export is deleted"))
	}

	return export, nil
}

// Delete performs hard delete to be consistent with PostgreSQL implementation
func (r *ExportRegistry) Delete(ctx context.Context, id string) error {
	export, err := r.Registry.Get(ctx, id)
	if err != nil {
		return err
	}

	if export.IsDeleted() {
		return errxtrace.Classify(registry.ErrDeleted, errx.Attrs("reason", "export already deleted"))
	}

	// Hard delete the export
	return r.Registry.Delete(ctx, id)
}

// Count returns count of non-deleted exports
func (r *ExportRegistry) Count(ctx context.Context) (int, error) {
	allExports, err := r.Registry.List(ctx)
	if err != nil {
		return 0, err
	}

	// Count only non-deleted exports
	count := 0
	for _, export := range allExports {
		if !export.IsDeleted() {
			count++
		}
	}

	return count, nil
}

// ListWithDeleted returns all exports including soft deleted ones
func (r *ExportRegistry) ListWithDeleted(ctx context.Context) ([]*models.Export, error) {
	return r.Registry.List(ctx)
}

// ListDeleted returns only soft deleted exports
func (r *ExportRegistry) ListDeleted(ctx context.Context) ([]*models.Export, error) {
	allExports, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}

	// Filter for deleted exports only
	var deletedExports []*models.Export
	for _, export := range allExports {
		if export.IsDeleted() {
			deletedExports = append(deletedExports, export)
		}
	}

	return deletedExports, nil
}

// HardDelete permanently deletes an export from the database
func (r *ExportRegistry) HardDelete(ctx context.Context, id string) error {
	return r.Registry.Delete(ctx, id)
}
