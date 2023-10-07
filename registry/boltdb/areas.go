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
		db:               db,
		base:             base,
		registry:         NewRegistry[models.Area, *models.Area](db, base),
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
	err = r.db.View(func(tx *bolt.Tx) error {
		entity := &models.Area{}
		err := r.base.GetByIndexValue(tx, idxAreasByName, name, entity)
		if err != nil {
			return err
		}
		result = entity
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
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
		children := r.base.GetBucket(tx, bucketNameAreasChildren, area.ID)
		if children != nil {
			bucket := r.base.GetBucket(children, area.ID)
			vals, err := r.base.GetIndexValues(bucket, bucketNameCommodities)
			if err == nil && len(vals) > 0 {
				return errkit.Wrap(registry.ErrCannotDelete, "area has commodities")
			}
			if bucket != nil {
				_ = children.DeleteBucket([]byte(area.ID))
			}
		}
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Area) error {
		err := r.base.DeleteIndexValue(tx, idxAreasByName, result.Name)
		if err != nil {
			return err
		}

		return nil
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
	return r.db.Update(func(tx *bolt.Tx) error {
		m := &models.Area{}
		err := r.base.Get(tx, areaID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, bucketNameAreasChildren, m.ID)
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, "unknown area id")
		}

		err = r.base.SaveIndexValue(children, bucketNameCommodities, commodityID, commodityID)
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *AreaRegistry) GetCommodities(areaID string) ([]string, error) {
	var values map[string]string

	err := r.db.View(func(tx *bolt.Tx) error {
		m := &models.Area{}
		err := r.base.Get(tx, areaID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, bucketNameAreasChildren, m.ID)
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, "unknown area id")
		}

		values, err = r.base.GetIndexValues(children, bucketNameCommodities)
		if err != nil {
			return errkit.Wrap(err, "failed to get commodities")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	areas := make([]string, 0, len(values))

	for v := range values {
		areas = append(areas, v)
	}

	return areas, nil
}

func (r *AreaRegistry) DeleteCommodity(areaID, commodityID string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		m := &models.Area{}
		err := r.base.Get(tx, areaID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, bucketNameAreasChildren, m.ID)
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, "unknown area id")
		}

		err = r.base.DeleteIndexValue(children, bucketNameCommodities, commodityID)
		if err != nil {
			return err
		}

		return nil
	})
}
