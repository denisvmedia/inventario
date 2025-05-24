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

func (r *ManualRegistry) Create(ctx context.Context, m models.Manual) (*models.Manual, error) {
	result, err := r.registry.Create(m, func(_tx dbx.TransactionOrBucket, _manual *models.Manual) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _manual *models.Manual) error {
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create manual")
	}

	err = r.commodityRegistry.AddManual(ctx, result.CommodityID, result.ID)
	if err != nil {
		return result, errkit.Wrap(err, "failed to add manual to commodity")
	}

	return result, nil
}

func (r *ManualRegistry) Get(_ context.Context, id string) (*models.Manual, error) {
	return r.registry.Get(id)
}

func (r *ManualRegistry) List(_ context.Context) ([]*models.Manual, error) {
	return r.registry.List()
}

func (r *ManualRegistry) Update(_ context.Context, m models.Manual) (*models.Manual, error) {
	return r.registry.Update(m, func(_tx dbx.TransactionOrBucket, _manual *models.Manual) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _result *models.Manual) error {
		return nil
	})
}

func (r *ManualRegistry) Delete(ctx context.Context, id string) error {
	var commodityID string
	err := r.registry.Delete(id, func(_tx dbx.TransactionOrBucket, manual *models.Manual) error {
		commodityID = manual.CommodityID
		return nil
	}, func(_tx dbx.TransactionOrBucket, _result *models.Manual) error {
		return nil
	})
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual")
	}

	err = r.commodityRegistry.DeleteManual(ctx, commodityID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to remove manual from commodity")
	}

	return nil
}

func (r *ManualRegistry) Count(_ context.Context) (int, error) {
	return r.registry.Count()
}
