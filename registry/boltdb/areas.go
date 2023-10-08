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
	entityNameArea = "area"

	bucketNameAreas         = "areas"
	bucketNameAreasChildren = "areas-children"

	idxAreasByName = "areas-names"
)

var _ registry.AreaRegistry = (*AreaRegistry)(nil)

type AreaRegistry struct {
	db               *bolt.DB
	base             *dbx.BaseRepository[models.Area, *models.Area]
	registry         *Registry[models.Area, *models.Area]
	locationRegistry registry.LocationRegistry
}

func NewAreaRegistry(db *bolt.DB, locationRegistry registry.LocationRegistry) *AreaRegistry {
	base := dbx.NewBaseRepository[models.Area, *models.Area](bucketNameAreas)

	return &AreaRegistry{
		db:   db,
		base: base,
		registry: NewRegistry[models.Area, *models.Area](
			db,
			base,
			entityNameArea,
			bucketNameAreasChildren,
		),
		locationRegistry: locationRegistry,
	}
}

func (r *AreaRegistry) Create(m models.Area) (*models.Area, error) {
	result, err := r.registry.Create(m, func(tx dbx.TransactionOrBucket, area *models.Area) error {
		if area.Name == "" {
			return errkit.WithStack(registry.ErrFieldRequired,
				"field_name", "Name",
			)
		}

		_, err := r.base.GetIndexValue(tx, idxAreasByName, area.Name)
		if err == nil {
			return errkit.Wrap(registry.ErrAlreadyExists, "area name is already used")
		}
		if !errors.Is(err, registry.ErrNotFound) {
			// any other error is a problem
			return err
		}
		return nil
	}, func(tx dbx.TransactionOrBucket, area *models.Area) error {
		err := r.base.SaveIndexValue(tx, idxAreasByName, area.Name, area.ID)
		if err != nil {
			return err
		}

		r.base.GetOrCreateBucket(tx, bucketNameAreasChildren, area.ID)
		r.base.GetOrCreateBucket(tx, bucketNameAreasChildren, area.ID, bucketNameCommodities)

		return nil
	})
	if err != nil {
		return nil, err
	}

	err = r.locationRegistry.AddArea(result.LocationID, result.ID)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *AreaRegistry) Get(id string) (result *models.Area, err error) {
	return r.registry.Get(id)
}

func (r *AreaRegistry) GetOneByName(name string) (result *models.Area, err error) {
	return r.registry.GetBy(idxAreasByName, name)
}

func (r *AreaRegistry) List() (results []*models.Area, err error) {
	return r.registry.List()
}

func (r *AreaRegistry) Update(m models.Area) (result *models.Area, err error) {
	var old *models.Area
	return r.registry.Update(m, func(tx dbx.TransactionOrBucket, area *models.Area) error {
		old = area
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Area) error {
		if old.Name == result.Name {
			return nil
		}

		u := &models.Area{}
		err := r.base.GetByIndexValue(tx, idxAreasByName, result.Name, u)
		switch {
		case err == nil:
			return errkit.Wrap(registry.ErrAlreadyExists, "area name is already used")
		case errors.Is(err, registry.ErrNotFound):
			// skip, it's expected
		case err != nil:
			return errkit.Wrap(err, "failed to check if area name is already used")
		}

		err = r.base.DeleteIndexValue(tx, idxAreasByName, old.Name)
		if err != nil {
			return err
		}
		err = r.base.SaveIndexValue(tx, idxAreasByName, result.Name, result.GetID())
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *AreaRegistry) Count() (int, error) {
	return r.registry.Count()
}

func (r *AreaRegistry) Delete(id string) error {
	var locationID string
	err := r.registry.Delete(id, func(tx dbx.TransactionOrBucket, area *models.Area) error {
		locationID = area.LocationID
		return r.registry.DeleteEmptyBuckets(
			tx,
			area.ID,
			bucketNameCommodities,
		)
	}, func(tx dbx.TransactionOrBucket, result *models.Area) error {
		return r.base.DeleteIndexValue(tx, idxAreasByName, result.Name)
	})
	if err != nil {
		return err
	}

	err = r.locationRegistry.DeleteArea(locationID, id)
	if err != nil {
		return err
	}

	return nil
}

func (r *AreaRegistry) AddCommodity(areaID, commodityID string) error {
	return r.registry.AddChild(bucketNameCommodities, areaID, commodityID)
}

func (r *AreaRegistry) GetCommodities(areaID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameCommodities, areaID)
}

func (r *AreaRegistry) DeleteCommodity(areaID, commodityID string) error {
	return r.registry.DeleteChild(bucketNameCommodities, areaID, commodityID)
}
