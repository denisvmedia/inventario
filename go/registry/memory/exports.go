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
	// restoreOperationFactory lets Delete clear the restore pipeline that
	// references an export before the export row is removed, mirroring the
	// postgres backend (#2118).
	restoreOperationFactory *RestoreOperationRegistryFactory
}

// ExportRegistry is a context-aware registry that can only be created through the factory
type ExportRegistry struct {
	*Registry[models.Export, *models.Export]

	restoreOperationFactory *RestoreOperationRegistryFactory
	userID                  string
}

var _ registry.ExportRegistry = (*ExportRegistry)(nil)
var _ registry.ExportRegistryFactory = (*ExportRegistryFactory)(nil)

func NewExportRegistryFactory(restoreOperationFactory *RestoreOperationRegistryFactory) *ExportRegistryFactory {
	return &ExportRegistryFactory{
		baseExportRegistry:      NewRegistry[models.Export, *models.Export](),
		restoreOperationFactory: restoreOperationFactory,
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
	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := &Registry[models.Export, *models.Export]{
		items:   f.baseExportRegistry.items, // Share the data map
		lock:    f.baseExportRegistry.lock,  // Share the mutex pointer
		userID:  user.ID,                    // Set user-specific userID
		groupID: groupID,                    // Set group-specific groupID
	}

	return &ExportRegistry{
		Registry:                userRegistry,
		restoreOperationFactory: f.restoreOperationFactory,
		userID:                  user.ID,
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
		Registry:                serviceRegistry,
		restoreOperationFactory: f.restoreOperationFactory,
		userID:                  "", // Clear userID to bypass user filtering
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
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("reason", "export already deleted"))
	}

	// Clear the restore pipeline that references this export before the
	// export row is removed, mirroring the postgres ExportRegistry.Delete
	// (#2118). Postgres MUST do this because restore_operations.export_id is
	// a NOT NULL NO ACTION FK; memory has no FK enforcement, but the two
	// backends have to behave identically — otherwise the in-memory store is
	// left with restore operations orphaned to a deleted export, and a
	// memory-backed test of export deletion would pass regardless of the
	// postgres cleanup. RestoreOperationRegistry.Delete cascades to the
	// operation's steps; a service-scoped registry is used because
	// ListByExport already bounds the work to this (globally unique) export.
	//
	// Two intentional differences from the postgres path, both benign here:
	// (1) postgres additionally bounds its DELETEs by the transaction's
	// tenant+group RLS GUCs, whereas memory relies solely on the unique
	// export id plus the user-scoped ownership gate above — equivalent
	// because a restore op always co-resides in its export's group. (2) The
	// postgres delete is a single transaction; this loop is best-effort and
	// non-atomic. In practice it cannot partially fail: the memory
	// RestoreOperationRegistry.Delete only ever returns nil/ErrNotFound for a
	// row ListByExport just returned, so there is no mid-loop rollback case.
	restoreOps := r.restoreOperationFactory.CreateServiceRegistry()
	ops, err := restoreOps.ListByExport(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to list restore operations for export", err)
	}
	for _, op := range ops {
		if err := restoreOps.Delete(ctx, op.ID); err != nil {
			return errxtrace.Wrap("failed to delete restore operation for export", err)
		}
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
