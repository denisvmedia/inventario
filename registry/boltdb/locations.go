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
	bucketNameLocations         = "locations"
	bucketNameLocationsChildren = "locations-children"

	idxLocationsByName = "locations-names"
)

var _ registry.LocationRegistry = (*LocationRegistry)(nil)

type LocationRegistry struct {
	db       *bolt.DB
	base     *dbx.BaseRepository[models.Location, *models.Location]
	registry *Registry[models.Location, *models.Location]
}

func NewLocationRegistry(db *bolt.DB) *LocationRegistry {
	base := dbx.NewBaseRepository[models.Location, *models.Location](bucketNameLocations)

	return &LocationRegistry{
		db:       db,
		base:     base,
		registry: NewRegistry[models.Location, *models.Location](db, base),
	}
}

func (r *LocationRegistry) Create(m models.Location) (*models.Location, error) {
	return r.registry.Create(m, func(tx dbx.TransactionOrBucket, location *models.Location) error {
		if location.Name == "" {
			return errkit.WithStack(registry.ErrFieldRequired,
				"field_name", "Name",
			)
		}

		_, err := r.base.GetIndexValue(tx, idxLocationsByName, location.Name)
		if err == nil {
			return errkit.Wrap(registry.ErrAlreadyExists, "user name is already used")
		}
		if !errors.Is(err, registry.ErrNotFound) {
			// any other error is a problem
			return err
		}
		return nil
	}, func(tx dbx.TransactionOrBucket, location *models.Location) error {
		err := r.base.SaveIndexValue(tx, idxLocationsByName, location.Name, location.ID)
		if err != nil {
			return err
		}

		r.base.GetOrCreateBucket(tx, bucketNameLocationsChildren, location.ID)
		r.base.GetOrCreateBucket(tx, bucketNameLocationsChildren, location.ID, bucketNameAreas)

		return nil
	})
}

func (r *LocationRegistry) Get(id string) (result *models.Location, err error) {
	return r.registry.Get(id)
}

func (r *LocationRegistry) GetOneByName(name string) (result *models.Location, err error) {
	err = r.db.View(func(tx *bolt.Tx) error {
		entity := &models.Location{}
		err := r.base.GetByIndexValue(tx, idxLocationsByName, name, entity)
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

func (r *LocationRegistry) List() (results []*models.Location, err error) {
	return r.registry.List()
}

func (r *LocationRegistry) Update(m models.Location) (result *models.Location, err error) {
	var old *models.Location
	return r.registry.Update(m, func(tx dbx.TransactionOrBucket, location *models.Location) error {
		old = location
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Location) error {
		if old.Name == result.Name {
			return nil
		}

		u := &models.Location{}
		err := r.base.GetByIndexValue(tx, idxLocationsByName, result.Name, u)
		switch {
		case err == nil:
			return errkit.Wrap(registry.ErrAlreadyExists, "location name is already used")
		case errors.Is(err, registry.ErrNotFound):
			// skip, it's expected
		case err != nil:
			return errkit.Wrap(err, "failed to check if location name is already used")
		}

		err = r.base.DeleteIndexValue(tx, idxLocationsByName, old.Name)
		if err != nil {
			return err
		}
		err = r.base.SaveIndexValue(tx, idxLocationsByName, result.Name, result.GetID())
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *LocationRegistry) Count() (int, error) {
	return r.registry.Count()
}

func (r *LocationRegistry) Delete(id string) error {
	return r.registry.Delete(id, func(tx dbx.TransactionOrBucket, location *models.Location) error {
		children := r.base.GetBucket(tx, bucketNameLocationsChildren, location.ID)
		if children != nil {
			bucket := r.base.GetBucket(children, location.ID)
			vals, err := r.base.GetIndexValues(bucket, bucketNameAreas)
			if err == nil && len(vals) > 0 {
				return errkit.Wrap(registry.ErrCannotDelete, "location has areas")
			}
			if bucket != nil {
				_ = children.DeleteBucket([]byte(location.ID))
			}
		}
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Location) error {
		err := r.base.DeleteIndexValue(tx, idxLocationsByName, result.Name)
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *LocationRegistry) AddArea(locationID, areaID string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		m := &models.Location{}
		err := r.base.Get(tx, locationID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, bucketNameLocationsChildren, m.ID)
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, "unknown location id")
		}

		err = r.base.SaveIndexValue(children, bucketNameAreas, areaID, areaID)
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *LocationRegistry) GetAreas(locationID string) ([]string, error) {
	var values map[string]string

	err := r.db.View(func(tx *bolt.Tx) error {
		m := &models.Location{}
		err := r.base.Get(tx, locationID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, bucketNameLocationsChildren, m.ID)
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, "unknown location id")
		}

		values, err = r.base.GetIndexValues(children, bucketNameAreas)
		if err != nil {
			return errkit.Wrap(err, "failed to get areas")
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

func (r *LocationRegistry) DeleteArea(locationID, areaID string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		m := &models.Location{}
		err := r.base.Get(tx, locationID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, bucketNameLocationsChildren, m.ID)
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, "unknown location id")
		}

		err = r.base.DeleteIndexValue(children, bucketNameAreas, areaID)
		if err != nil {
			return err
		}

		return nil
	})
}
