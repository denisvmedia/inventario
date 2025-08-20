package boltdb

import (
	"context"
	"errors"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameCommodity = "commodity"

	bucketNameCommodities         = "commodities"
	bucketNameCommoditiesChildren = "commodities-children"

	idxCommoditiesByName = "commodities-names"
)

var _ registry.CommodityRegistry = (*CommodityRegistry)(nil)

type CommodityRegistry struct {
	db           *bolt.DB
	base         *dbx.BaseRepository[models.Commodity, *models.Commodity]
	registry     *Registry[models.Commodity, *models.Commodity]
	areaRegistry registry.AreaRegistry
}

func NewCommodityRegistry(db *bolt.DB, areaRegistry registry.AreaRegistry) *CommodityRegistry {
	base := dbx.NewBaseRepository[models.Commodity, *models.Commodity](bucketNameCommodities)

	return &CommodityRegistry{
		db:   db,
		base: base,
		registry: NewRegistry[models.Commodity, *models.Commodity](
			db,
			base,
			entityNameCommodity,
			bucketNameCommoditiesChildren,
		),
		areaRegistry: areaRegistry,
	}
}

func (r *CommodityRegistry) Create(ctx context.Context, m models.Commodity) (*models.Commodity, error) {
	result, err := r.registry.Create(m, func(tx dbx.TransactionOrBucket, commodity *models.Commodity) error {
		if commodity.Name == "" {
			return errkit.WithStack(registry.ErrFieldRequired,
				"field_name", "Name",
			)
		}

		_, err := r.base.GetIndexValue(tx, idxCommoditiesByName, commodity.Name)
		if err == nil {
			return errkit.Wrap(registry.ErrAlreadyExists, "commodity name is already used")
		}
		if !errors.Is(err, registry.ErrNotFound) {
			// any other error is a problem
			return errkit.Wrap(err, "failed to check if commodity name is already used")
		}
		return nil
	}, func(tx dbx.TransactionOrBucket, commodity *models.Commodity) error {
		err := r.base.SaveIndexValue(tx, idxCommoditiesByName, commodity.Name, commodity.ID)
		if err != nil {
			return errkit.Wrap(err, "failed to save commodity name")
		}

		r.base.GetOrCreateBucket(tx, bucketNameCommoditiesChildren, commodity.ID)
		r.base.GetOrCreateBucket(tx, bucketNameCommoditiesChildren, commodity.ID, bucketNameImages)
		r.base.GetOrCreateBucket(tx, bucketNameCommoditiesChildren, commodity.ID, bucketNameInvoices)
		r.base.GetOrCreateBucket(tx, bucketNameCommoditiesChildren, commodity.ID, bucketNameImages)

		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create commodity")
	}

	err = r.areaRegistry.AddCommodity(ctx, result.AreaID, result.ID)
	if err != nil {
		return result, errkit.Wrap(err, "failed to add commodity to area")
	}

	return result, nil
}

func (r *CommodityRegistry) Get(_ context.Context, id string) (*models.Commodity, error) {
	return r.registry.Get(id)
}

func (r *CommodityRegistry) GetOneByName(_ context.Context, name string) (result *models.Commodity, err error) {
	return r.registry.GetBy(idxCommoditiesByName, name)
}

func (r *CommodityRegistry) List(_ context.Context) ([]*models.Commodity, error) {
	return r.registry.List()
}

func (r *CommodityRegistry) Update(_ context.Context, m models.Commodity) (*models.Commodity, error) {
	var old *models.Commodity
	return r.registry.Update(m, func(_tx dbx.TransactionOrBucket, commodity *models.Commodity) error {
		old = commodity
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Commodity) error {
		if old.Name == result.Name {
			return nil
		}

		u := &models.Commodity{}
		err := r.base.GetByIndexValue(tx, idxCommoditiesByName, result.Name, u)
		switch {
		case err == nil:
			return errkit.Wrap(registry.ErrAlreadyExists, "commodity name is already used")
		case errors.Is(err, registry.ErrNotFound):
			// skip, it's expected
		case err != nil:
			return errkit.Wrap(err, "failed to check if commodity name is already used")
		}

		err = r.base.DeleteIndexValue(tx, idxCommoditiesByName, old.Name)
		if err != nil {
			return errkit.Wrap(err, "failed to delete old commodity name")
		}
		err = r.base.SaveIndexValue(tx, idxCommoditiesByName, result.Name, result.GetID())
		if err != nil {
			return errkit.Wrap(err, "failed to save new commodity name")
		}

		return nil
	})
}

func (r *CommodityRegistry) Delete(ctx context.Context, id string) error {
	var areaID string
	err := r.registry.Delete(id, func(tx dbx.TransactionOrBucket, commodity *models.Commodity) error {
		areaID = commodity.AreaID
		return r.registry.DeleteEmptyBuckets(
			tx,
			commodity.ID,
			bucketNameImages,
			bucketNameInvoices,
			bucketNameManuals,
		)
	}, func(tx dbx.TransactionOrBucket, result *models.Commodity) error {
		return r.base.DeleteIndexValue(tx, idxCommoditiesByName, result.Name)
	})
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity")
	}

	err = r.areaRegistry.DeleteCommodity(ctx, areaID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity from area")
	}

	return nil
}

func (r *CommodityRegistry) Count(_ context.Context) (int, error) {
	return r.registry.Count()
}

func (r *CommodityRegistry) AddImage(_ context.Context, commodityID, imageID string) error {
	return r.registry.AddChild(bucketNameImages, commodityID, imageID)
}

func (r *CommodityRegistry) GetImages(_ context.Context, commodityID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameImages, commodityID)
}

func (r *CommodityRegistry) DeleteImage(_ context.Context, commodityID, imageID string) error {
	return r.registry.DeleteChild(bucketNameImages, commodityID, imageID)
}

func (r *CommodityRegistry) AddManual(_ context.Context, commodityID, manualID string) error {
	return r.registry.AddChild(bucketNameManuals, commodityID, manualID)
}

func (r *CommodityRegistry) GetManuals(_ context.Context, commodityID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameManuals, commodityID)
}

func (r *CommodityRegistry) DeleteManual(_ context.Context, commodityID, manualID string) error {
	return r.registry.DeleteChild(bucketNameManuals, commodityID, manualID)
}

func (r *CommodityRegistry) AddInvoice(_ context.Context, commodityID, invoiceID string) error {
	return r.registry.AddChild(bucketNameInvoices, commodityID, invoiceID)
}

func (r *CommodityRegistry) GetInvoices(_ context.Context, commodityID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameInvoices, commodityID)
}

func (r *CommodityRegistry) DeleteInvoice(_ context.Context, commodityID, invoiceID string) error {
	return r.registry.DeleteChild(bucketNameInvoices, commodityID, invoiceID)
}

// User-aware methods that delegate to the embedded registry
func (r *CommodityRegistry) SetUserContext(ctx context.Context, userID string) error {
	return r.registry.SetUserContext(ctx, userID)
}

func (r *CommodityRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	return r.registry.WithUserContext(ctx, userID, fn)
}

func (r *CommodityRegistry) CreateWithUser(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	return r.registry.CreateWithUser(ctx, commodity)
}

func (r *CommodityRegistry) GetWithUser(ctx context.Context, id string) (*models.Commodity, error) {
	return r.registry.GetWithUser(ctx, id)
}

func (r *CommodityRegistry) ListWithUser(ctx context.Context) ([]*models.Commodity, error) {
	return r.registry.ListWithUser(ctx)
}

func (r *CommodityRegistry) UpdateWithUser(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	return r.registry.UpdateWithUser(ctx, commodity)
}

func (r *CommodityRegistry) DeleteWithUser(ctx context.Context, id string) error {
	return r.registry.DeleteWithUser(ctx, id)
}

func (r *CommodityRegistry) CountWithUser(ctx context.Context) (int, error) {
	return r.registry.CountWithUser(ctx)
}
