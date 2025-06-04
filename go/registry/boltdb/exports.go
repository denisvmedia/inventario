package boltdb

import (
	"context"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameExport = "export"
	bucketNameExports = "exports"
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
		func(dbx.TransactionOrBucket, *models.Export) error { return nil }, 
		func(dbx.TransactionOrBucket, *models.Export) error { return nil })
}

func (r *ExportRegistry) Get(ctx context.Context, id string) (*models.Export, error) {
	return r.registry.Get(id)
}

func (r *ExportRegistry) List(ctx context.Context) ([]*models.Export, error) {
	return r.registry.List()
}

func (r *ExportRegistry) Update(ctx context.Context, export models.Export) (*models.Export, error) {
	return r.registry.Update(export, 
		func(dbx.TransactionOrBucket, *models.Export) error { return nil }, 
		func(dbx.TransactionOrBucket, *models.Export) error { return nil })
}

func (r *ExportRegistry) Delete(ctx context.Context, id string) error {
	return r.registry.Delete(id, 
		func(dbx.TransactionOrBucket, *models.Export) error { return nil }, 
		func(dbx.TransactionOrBucket, *models.Export) error { return nil })
}

func (r *ExportRegistry) Count(ctx context.Context) (int, error) {
	return r.registry.Count()
}