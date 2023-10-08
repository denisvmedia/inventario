package boltdb

import (
	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameManual = "manual"

	bucketNameManuals         = "manuals"
	bucketNameManualsChildren = "manuals-children"
)

var _ registry.ManualRegistry = (*ManualRegistry)(nil)

type ManualRegistry struct {
	db                *bolt.DB
	base              *dbx.BaseRepository[models.Manual, *models.Manual]
	registry          *Registry[models.Manual, *models.Manual]
	commodityRegistry registry.CommodityRegistry
}

func NewManualRegistry(db *bolt.DB, commodityRegistry registry.CommodityRegistry) *ManualRegistry {
	base := dbx.NewBaseRepository[models.Manual, *models.Manual](bucketNameManuals)

	return &ManualRegistry{
		db:   db,
		base: base,
		registry: NewRegistry[models.Manual, *models.Manual](
			db,
			base,
			entityNameManual,
			bucketNameManualsChildren,
		),
		commodityRegistry: commodityRegistry,
	}
}

func (r *ManualRegistry) Create(m models.Manual) (*models.Manual, error) {
	result, err := r.registry.Create(m, func(tx dbx.TransactionOrBucket, manual *models.Manual) error {
		return nil
	}, func(tx dbx.TransactionOrBucket, manual *models.Manual) error {
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = r.commodityRegistry.AddManual(result.CommodityID, result.ID)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *ManualRegistry) Get(id string) (*models.Manual, error) {
	return r.registry.Get(id)
}

func (r *ManualRegistry) List() ([]*models.Manual, error) {
	return r.registry.List()
}

func (r *ManualRegistry) Update(m models.Manual) (*models.Manual, error) {
	return r.registry.Update(m, func(tx dbx.TransactionOrBucket, manual *models.Manual) error {
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Manual) error {
		return nil
	})
}

func (r *ManualRegistry) Delete(id string) error {
	var commodityID string
	err := r.registry.Delete(id, func(tx dbx.TransactionOrBucket, manual *models.Manual) error {
		commodityID = manual.CommodityID
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Manual) error {
		return nil
	})
	if err != nil {
		return err
	}

	err = r.commodityRegistry.DeleteManual(commodityID, id)
	if err != nil {
		return err
	}

	return nil
}

func (r *ManualRegistry) Count() (int, error) {
	return r.registry.Count()
}
