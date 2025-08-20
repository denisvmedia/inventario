package boltdb

import (
	"context"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameExport          = "export"
	bucketNameExports         = "exports"
	bucketNameExportsChildren = "exports-children"
)

var _ registry.ExportRegistry = (*ExportRegistry)(nil)

type ExportRegistry struct {
	db       *bolt.DB
	base     *dbx.BaseRepository[models.Export, *models.Export]
	registry *Registry[models.Export, *models.Export]
}

func NewExportRegistry(db *bolt.DB) registry.ExportRegistry {
	base := dbx.NewBaseRepository[models.Export, *models.Export](entityNameExport)
	return &ExportRegistry{
		db:       db,
		base:     base,
		registry: NewRegistry(db, base, entityNameExport, bucketNameExportsChildren),
	}
}

func (r *ExportRegistry) Create(ctx context.Context, export models.Export) (*models.Export, error) {
	return r.registry.Create(export,
		func(tx dbx.TransactionOrBucket, e *models.Export) error {
			// Validate required fields
			if e.Description == "" {
				return errkit.WithStack(registry.ErrFieldRequired, "field_name", "Description")
			}
			if e.Type == "" {
				return errkit.WithStack(registry.ErrFieldRequired, "field_name", "Type")
			}

			// Set default status if not set
			if e.Status == "" {
				e.Status = models.ExportStatusPending
			}

			return nil
		},
		func(dbx.TransactionOrBucket, *models.Export) error { return nil },
	)
}

func (r *ExportRegistry) Get(ctx context.Context, id string) (*models.Export, error) {
	return r.registry.Get(id)
}

func (r *ExportRegistry) List(ctx context.Context) ([]*models.Export, error) {
	allExports, err := r.registry.List()
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

func (r *ExportRegistry) Update(ctx context.Context, export models.Export) (*models.Export, error) {
	return r.registry.Update(export,
		func(tx dbx.TransactionOrBucket, e *models.Export) error {
			// Validate required fields
			if e.Description == "" {
				return errkit.WithStack(registry.ErrFieldRequired, "field_name", "Description")
			}
			if e.Type == "" {
				return errkit.WithStack(registry.ErrFieldRequired, "field_name", "Type")
			}

			return nil
		},
		func(dbx.TransactionOrBucket, *models.Export) error { return nil },
	)
}

func (r *ExportRegistry) Delete(ctx context.Context, id string) error {
	// Get the export first to check if it exists
	export, err := r.registry.Get(id)
	if err != nil {
		return err
	}

	if export.IsDeleted() {
		return errkit.WithStack(registry.ErrNotFound, "export already deleted")
	}

	// Hard delete the export
	return r.registry.Delete(id,
		func(dbx.TransactionOrBucket, *models.Export) error { return nil },
		func(dbx.TransactionOrBucket, *models.Export) error { return nil },
	)
}

func (r *ExportRegistry) Count(ctx context.Context) (int, error) {
	allExports, err := r.registry.List()
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
	return r.registry.List()
}

// ListDeleted returns only soft deleted exports
func (r *ExportRegistry) ListDeleted(ctx context.Context) ([]*models.Export, error) {
	allExports, err := r.registry.List()
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
	return r.registry.Delete(id,
		func(dbx.TransactionOrBucket, *models.Export) error { return nil },
		func(dbx.TransactionOrBucket, *models.Export) error { return nil },
	)
}

// User-aware methods that delegate to the embedded registry
func (r *ExportRegistry) SetUserContext(ctx context.Context, userID string) error {
	return r.registry.SetUserContext(ctx, userID)
}

func (r *ExportRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	return r.registry.WithUserContext(ctx, userID, fn)
}

func (r *ExportRegistry) CreateWithUser(ctx context.Context, export models.Export) (*models.Export, error) {
	return r.registry.CreateWithUser(ctx, export)
}

func (r *ExportRegistry) GetWithUser(ctx context.Context, id string) (*models.Export, error) {
	return r.registry.GetWithUser(ctx, id)
}

func (r *ExportRegistry) ListWithUser(ctx context.Context) ([]*models.Export, error) {
	return r.registry.ListWithUser(ctx)
}

func (r *ExportRegistry) UpdateWithUser(ctx context.Context, export models.Export) (*models.Export, error) {
	return r.registry.UpdateWithUser(ctx, export)
}

func (r *ExportRegistry) DeleteWithUser(ctx context.Context, id string) error {
	return r.registry.DeleteWithUser(ctx, id)
}

func (r *ExportRegistry) CountWithUser(ctx context.Context) (int, error) {
	return r.registry.CountWithUser(ctx)
}
