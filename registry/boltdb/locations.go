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
	entityNameLocation = "location"

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
		db:   db,
		base: base,
		registry: NewRegistry[models.Location, *models.Location](
			db,
			base,
			entityNameLocation,
			bucketNameLocationsChildren,
		),
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
			return errkit.Wrap(registry.ErrAlreadyExists, "location name is already used")
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
	return r.registry.GetBy(idxLocationsByName, name)
}

func (r *LocationRegistry) List() (results []*models.Location, err error) {
	return r.registry.List()
}

func (r *LocationRegistry) Update(m models.Location) (result *models.Location, err error) {
	var old *models.Location
	return r.registry.Update(m, func(_tx dbx.TransactionOrBucket, location *models.Location) error {
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
		return r.registry.DeleteEmptyBuckets(
			tx,
			location.ID,
			bucketNameAreas,
		)
	}, func(tx dbx.TransactionOrBucket, result *models.Location) error {
		return r.base.DeleteIndexValue(tx, idxLocationsByName, result.Name)
	})
}

func (r *LocationRegistry) AddArea(locationID, areaID string) error {
	return r.registry.AddChild(bucketNameAreas, locationID, areaID)
}

func (r *LocationRegistry) GetAreas(locationID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameAreas, locationID)
}

func (r *LocationRegistry) DeleteArea(locationID, areaID string) error {
	return r.registry.DeleteChild(bucketNameAreas, locationID, areaID)
}
