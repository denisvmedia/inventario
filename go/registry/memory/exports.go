package memory

import (
	"context"
	"time"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type ExportRegistry struct {
	*Registry[models.Export, *models.Export]
}

func NewExportRegistry() registry.ExportRegistry {
	return &ExportRegistry{
		Registry: NewRegistry[models.Export, *models.Export](),
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

// Delete performs soft delete by setting deleted_at timestamp
func (r *ExportRegistry) Delete(ctx context.Context, id string) error {
	export, err := r.Registry.Get(ctx, id)
	if err != nil {
		return err
	}

	if export.IsDeleted() {
		return errkit.WithStack(registry.ErrNotFound, "export already deleted")
	}

	// Set deleted_at timestamp
	now := models.Date(time.Now().Format("2006-01-02"))
	export.DeletedAt = &now

	_, err = r.Registry.Update(ctx, *export)
	return err
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
