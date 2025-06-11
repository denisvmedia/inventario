package commonsql

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.LocationRegistry = (*LocationRegistry)(nil)

type LocationRegistry struct {
	dbx          *sqlx.DB
	tableNames   TableNames
	areaRegistry registry.AreaRegistry
}

func NewLocationRegistry(dbx *sqlx.DB) *LocationRegistry {
	return NewLocationRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewLocationRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *LocationRegistry {
	return &LocationRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// SetAreaRegistry sets the area registry for recursive deletion
func (r *LocationRegistry) SetAreaRegistry(areaRegistry registry.AreaRegistry) {
	r.areaRegistry = areaRegistry
}

func (r *LocationRegistry) Create(ctx context.Context, location models.Location) (*models.Location, error) {
	if location.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	// Generate a new ID
	location.SetID(generateID())

	// Insert the location into the database (atomic operation)
	err := InsertEntity(ctx, r.dbx, r.tableNames.Locations(), location)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &location, nil
}

func (r *LocationRegistry) Get(ctx context.Context, id string) (*models.Location, error) {
	// Query the database for the location (atomic operation)
	return r.get(ctx, r.dbx, id)
}

func (r *LocationRegistry) get(ctx context.Context, tx sqlx.ExtContext, id string) (*models.Location, error) {
	var location models.Location
	err := ScanEntityByField(ctx, tx, r.tableNames.Locations(), "id", id, &location)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get location")
	}

	return &location, nil
}

func (r *LocationRegistry) List(ctx context.Context) ([]*models.Location, error) {
	var locations []*models.Location

	// Query the database for all locations (atomic operation)
	for location, err := range ScanEntities[models.Location](ctx, r.dbx, r.tableNames.Locations()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list locations")
		}
		locations = append(locations, &location)
	}

	return locations, nil
}

func (r *LocationRegistry) Update(ctx context.Context, location models.Location) (*models.Location, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the location exists
	_, err = r.get(ctx, tx, location.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get location")
	}

	err = UpdateEntityByField(ctx, tx, r.tableNames.Locations(), "id", location.ID, location)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update location")
	}

	return &location, nil
}

func (r *LocationRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the location exists
	_, err = r.get(ctx, tx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get location")
	}

	// Check if the location has areas
	areas, err := r.getAreas(ctx, tx, id)
	if err != nil {
		return err
	}
	if len(areas) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "area has areas")
	}

	// Finally, delete the location
	err = DeleteEntityByField(ctx, tx, r.tableNames.Locations(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete location")
	}

	return nil
}

func (r *LocationRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := CountEntities(ctx, r.dbx, r.tableNames.Locations())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count locations")
	}

	return cnt, nil
}

func (r *LocationRegistry) AddArea(ctx context.Context, locationID, areaID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the location exists
	_, err = r.get(ctx, tx, locationID)
	if err != nil {
		return errkit.Wrap(err, "failed to get location")
	}

	// Check if the area exists
	var area models.Area
	err = ScanEntityByField(ctx, tx, r.tableNames.Areas(), "id", areaID, &area)
	if err != nil {
		return errkit.Wrap(err, "failed to get area")
	}

	// Check if the area is already in the location
	if area.LocationID == locationID {
		// already in location id
		return nil
	}

	// Set the area's location ID and update it
	area.LocationID = locationID
	err = UpdateEntityByField(ctx, tx, r.tableNames.Areas(), "id", areaID, area)
	if err != nil {
		return errkit.Wrap(err, "failed to update area")
	}

	return nil
}

func (r *LocationRegistry) GetAreas(ctx context.Context, locationID string) ([]string, error) {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	return r.getAreas(ctx, tx, locationID)
}

func (r *LocationRegistry) getAreas(ctx context.Context, tx sqlx.ExtContext, locationID string) ([]string, error) {
	// Check if the location exists
	_, err := r.get(ctx, tx, locationID)
	if err != nil {
		return nil, err
	}

	var areas []string

	for area, err := range ScanEntitiesByField[models.Area](ctx, tx, r.tableNames.Areas(), "location_id", locationID) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list areas")
		}
		areas = append(areas, area.GetID())
	}

	return areas, nil
}

func (r *LocationRegistry) DeleteArea(ctx context.Context, locationID, areaID string) (err error) {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the location exists
	_, err = r.get(ctx, tx, locationID)
	if err != nil {
		return err
	}

	var area models.Area
	err = ScanEntityByField(ctx, tx, r.tableNames.Areas(), "id", areaID, &area)
	if err != nil {
		return errkit.Wrap(err, "failed to get area")
	}

	if area.LocationID != locationID {
		return errkit.Wrap(registry.ErrNotFound, "area not found or does not belong to this location")
	}

	err = DeleteEntityByField(ctx, tx, r.tableNames.Areas(), "id", areaID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area")
	}

	return nil
}

// DeleteRecursive deletes a location and all its areas and commodities recursively
func (r *LocationRegistry) DeleteRecursive(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the location exists
	_, err = r.get(ctx, tx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get location")
	}

	// Get all areas in this location
	areas, err := r.getAreas(ctx, tx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get areas")
	}

	// Delete all areas recursively (this will also delete their commodities)
	for _, areaID := range areas {
		if r.areaRegistry != nil {
			if err := r.areaRegistry.DeleteRecursive(ctx, areaID); err != nil {
				// If the area is already deleted, that's fine - continue with others
				if !errors.Is(err, registry.ErrNotFound) {
					return errkit.Wrap(err, fmt.Sprintf("failed to delete area %s recursively", areaID))
				}
			}
		}
	}

	// Finally, delete the location
	err = DeleteEntityByField(ctx, tx, r.tableNames.Locations(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete location")
	}

	return nil
}
