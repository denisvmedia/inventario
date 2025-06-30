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



func (r *LocationRegistry) Create(_ context.Context, m models.Location) (*models.Location, error) {
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
			return errkit.Wrap(err, "failed to check if location name is already used")
		}
		return nil
	}, func(tx dbx.TransactionOrBucket, location *models.Location) error {
		err := r.base.SaveIndexValue(tx, idxLocationsByName, location.Name, location.ID)
		if err != nil {
			return errkit.Wrap(err, "failed to create location")
		}

		r.base.GetOrCreateBucket(tx, bucketNameLocationsChildren, location.ID)
		r.base.GetOrCreateBucket(tx, bucketNameLocationsChildren, location.ID, bucketNameAreas)

		return nil
	})
}

func (r *LocationRegistry) Get(_ context.Context, id string) (result *models.Location, err error) {
	return r.registry.Get(id)
}

func (r *LocationRegistry) GetOneByName(_ context.Context, name string) (result *models.Location, err error) {
	return r.registry.GetBy(idxLocationsByName, name)
}

func (r *LocationRegistry) List(_ context.Context) (results []*models.Location, err error) {
	return r.registry.List()
}

func (r *LocationRegistry) Update(_ context.Context, m models.Location) (result *models.Location, err error) {
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
			return errkit.Wrap(err, "failed to update location")
		}
		err = r.base.SaveIndexValue(tx, idxLocationsByName, result.Name, result.GetID())
		if err != nil {
			return errkit.Wrap(err, "failed to update location")
		}

		return nil
	})
}

func (r *LocationRegistry) Count(_ context.Context) (int, error) {
	return r.registry.Count()
}

func (r *LocationRegistry) Delete(ctx context.Context, id string) error {
	return r.registry.Delete(id, func(tx dbx.TransactionOrBucket, location *models.Location) error {
		areas, err := r.GetAreas(ctx, location.ID)
		if err != nil {
			return errkit.Wrap(err, "failed to get areas")
		}
		if len(areas) > 0 {
			return errkit.Wrap(registry.ErrCannotDelete, "location has areas")
		}

		return r.registry.DeleteEmptyBuckets(
			tx,
			location.ID,
			bucketNameAreas,
		)
	}, func(tx dbx.TransactionOrBucket, result *models.Location) error {
		return r.base.DeleteIndexValue(tx, idxLocationsByName, result.Name)
	})
}

func (r *LocationRegistry) AddArea(_ context.Context, locationID, areaID string) error {
	return r.registry.AddChild(bucketNameAreas, locationID, areaID)
}

func (r *LocationRegistry) GetAreas(_ context.Context, locationID string) ([]string, error) {
	return r.registry.GetChildren(bucketNameAreas, locationID)
}

func (r *LocationRegistry) DeleteArea(_ context.Context, locationID, areaID string) error {
	return r.registry.DeleteChild(bucketNameAreas, locationID, areaID)
}


