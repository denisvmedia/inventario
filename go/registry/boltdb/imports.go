package boltdb

import (
	"context"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameImport = "import"
	bucketNameImports = "imports"
	idxImportsByStatus = "imports-status"
)

var _ registry.ImportRegistry = (*ImportRegistry)(nil)

type ImportRegistry struct {
	db       *bolt.DB
	base     *dbx.BaseRepository[models.Import, *models.Import]
	registry *Registry[models.Import, *models.Import]
}

func NewImportRegistry(db *bolt.DB) *ImportRegistry {
	base := dbx.NewBaseRepository[models.Import, *models.Import](bucketNameImports)

	registry := NewRegistry[models.Import, *models.Import](db, base, entityNameImport, "")

	return &ImportRegistry{
		db:       db,
		base:     base,
		registry: registry,
	}
}

func (r *ImportRegistry) Create(ctx context.Context, import_ models.Import) (*models.Import, error) {
	return r.registry.Create(import_,
		func(tx dbx.TransactionOrBucket, i *models.Import) error {
			// Set default status if not set
			if i.Status == "" {
				i.Status = models.ImportStatusPending
			}
			return nil
		},
		func(dbx.TransactionOrBucket, *models.Import) error { return nil },
	)
}

func (r *ImportRegistry) Get(ctx context.Context, id string) (*models.Import, error) {
	return r.registry.Get(id)
}

func (r *ImportRegistry) List(ctx context.Context) ([]*models.Import, error) {
	return r.registry.List()
}

func (r *ImportRegistry) Update(ctx context.Context, import_ models.Import) (*models.Import, error) {
	return r.registry.Update(import_,
		func(dbx.TransactionOrBucket, *models.Import) error { return nil },
		func(dbx.TransactionOrBucket, *models.Import) error { return nil },
	)
}

func (r *ImportRegistry) Delete(ctx context.Context, id string) error {
	return r.registry.Delete(id,
		func(dbx.TransactionOrBucket, *models.Import) error { return nil },
		func(dbx.TransactionOrBucket, *models.Import) error { return nil },
	)
}

func (r *ImportRegistry) Count(ctx context.Context) (int, error) {
	return r.registry.Count()
}
