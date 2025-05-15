package boltdb

import (
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
	db               *bolt.DB
	base             *dbx.BaseRepository[models.Commodity, *models.Commodity]
	registry         *Registry[models.Commodity, *models.Commodity]
	areaRegistry     registry.AreaRegistry
	settingsRegistry registry.SettingsRegistry
}

func NewCommodityRegistry(db *bolt.DB, areaRegistry registry.AreaRegistry, settingsRegistry registry.SettingsRegistry) *CommodityRegistry {
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
		areaRegistry:     areaRegistry,
		settingsRegistry: settingsRegistry,
	}
}

func (r *CommodityRegistry) Create(m models.Commodity) (*models.Commodity, error) {
	err := registry.ValidateCommodity(&m, r.settingsRegistry)
	if err != nil {
		return nil, err
	}

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
			return err
		}
		return nil
	}, func(tx dbx.TransactionOrBucket, commodity *models.Commodity) error {
		err := r.base.SaveIndexValue(tx, idxCommoditiesByName, commodity.Name, commodity.ID)
		if err != nil {
			return err
		}

		r.base.GetOrCreateBucket(tx, bucketNameCommoditiesChildren, commodity.ID)
		r.base.GetOrCreateBucket(tx, bucketNameCommoditiesChildren, commodity.ID, bucketNameImages)
		r.base.GetOrCreateBucket(tx, bucketNameCommoditiesChildren, commodity.ID, bucketNameInvoices)
		r.base.GetOrCreateBucket(tx, bucketNameCommoditiesChildren, commodity.ID, bucketNameImages)

		return nil
	})
	if err != nil {
		return nil, err
	}

	err = r.areaRegistry.AddCommodity(result.AreaID, result.ID)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *CommodityRegistry) Get(id string) (*models.Commodity, error) {
	return r.registry.Get(id)
}

func (r *CommodityRegistry) GetOneByName(name string) (result *models.Commodity, err error) {
	return r.registry.GetBy(idxCommoditiesByName, name)
}

func (r *CommodityRegistry) List() ([]*models.Commodity, error) {
	return r.registry.List()
}

func (r *CommodityRegistry) Update(m models.Commodity) (*models.Commodity, error) {
	// Get main currency from settings
	settings, err := r.settingsRegistry.Get()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get settings")
	}

	// Default to USD if main currency is not set
	mainCurrency := "USD"
	if settings.MainCurrency != nil {
		mainCurrency = *settings.MainCurrency
	}

	// Validate that when original price is in the main currency, converted original price must be zero
	if string(m.OriginalPriceCurrency) == mainCurrency && !m.ConvertedOriginalPrice.IsZero() {
		return nil, errkit.Wrap(
			models.ErrConvertedPriceNotZero,
			"converted original price must be zero when original price is in the main currency",
		)
	}

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
			return err
		}
		err = r.base.SaveIndexValue(tx, idxCommoditiesByName, result.Name, result.GetID())
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *CommodityRegistry) Delete(id string) error {
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
		return err
	}

	err = r.areaRegistry.DeleteCommodity(areaID, id)
	if err != nil {
		return err
	}

	return nil
}

func (r *CommodityRegistry) Count() (int, error) {
	return r.registry.Count()
}

func (r *CommodityRegistry) AddImage(commodityID, imageID string) error {
	return r.registry.AddChild(bucketNameImages, commodityID, imageID)
}

func (r *CommodityRegistry) GetImages(commodityID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameImages, commodityID)
}

func (r *CommodityRegistry) DeleteImage(commodityID, imageID string) error {
	return r.registry.DeleteChild(bucketNameImages, commodityID, imageID)
}

func (r *CommodityRegistry) AddManual(commodityID, manualID string) error {
	return r.registry.AddChild(bucketNameManuals, commodityID, manualID)
}

func (r *CommodityRegistry) GetManuals(commodityID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameManuals, commodityID)
}

func (r *CommodityRegistry) DeleteManual(commodityID, manualID string) error {
	return r.registry.DeleteChild(bucketNameManuals, commodityID, manualID)
}

func (r *CommodityRegistry) AddInvoice(commodityID, invoiceID string) error {
	return r.registry.AddChild(bucketNameInvoices, commodityID, invoiceID)
}

func (r *CommodityRegistry) GetInvoices(commodityID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameInvoices, commodityID)
}

func (r *CommodityRegistry) DeleteInvoice(commodityID, invoiceID string) error {
	return r.registry.DeleteChild(bucketNameInvoices, commodityID, invoiceID)
}
