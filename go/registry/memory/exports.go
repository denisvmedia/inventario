package memory

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ExportRegistry = (*ExportRegistry)(nil)

type ExportRegistry struct {
	*Registry[models.Export, *models.Export]

	userID string
}

func NewExportRegistry() *ExportRegistry {
	return &ExportRegistry{
		Registry: NewRegistry[models.Export, *models.Export](),
	}
}

func (r *ExportRegistry) MustWithCurrentUser(ctx context.Context) registry.ExportRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *ExportRegistry) WithCurrentUser(ctx context.Context) (registry.ExportRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}
	tmp.userID = user.ID
	return &tmp, nil
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
		return export, errkit.WithStack(registry.ErrDeleted, "reason", "export is deleted")
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
		return errkit.WithStack(registry.ErrNotFound, "export already deleted")
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
