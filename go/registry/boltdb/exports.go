package boltdb

import (
	"context"
	"time"

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

			// Set created date if not set
			if e.CreatedDate == nil {
				now := models.Date(time.Now().Format("2006-01-02"))
				e.CreatedDate = &now
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
	_, err := r.registry.Update(models.Export{EntityID: models.EntityID{ID: id}},
		func(tx dbx.TransactionOrBucket, e *models.Export) error {
			// Check if already deleted
			if e.IsDeleted() {
				return errkit.WithStack(registry.ErrNotFound, "export already deleted")
			}

			// Set deleted_at timestamp
			now := models.Date(time.Now().Format("2006-01-02"))
			e.DeletedAt = &now
			return nil
		},
		func(dbx.TransactionOrBucket, *models.Export) error { return nil },
	)
	return err
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
