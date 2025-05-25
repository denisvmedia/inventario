package commonsql

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.AreaRegistry = (*AreaRegistry)(nil)

type AreaRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewAreaRegistry(dbx *sqlx.DB) *AreaRegistry {
	return NewAreaRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewAreaRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *AreaRegistry {
	return &AreaRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *AreaRegistry) Create(ctx context.Context, area models.Area) (*models.Area, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the location exists
	var location models.Location
	err = ScanEntityByField(ctx, tx, r.tableNames.Locations(), "id", area.LocationID, &location)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get location")
	}

	// Generate a new ID
	area.SetID(generateID())

	err = InsertEntity(ctx, tx, r.tableNames.Areas(), area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &area, nil
}

func (r *AreaRegistry) Get(ctx context.Context, id string) (*models.Area, error) {
	// Query the database for the area (atomic operation)
	return r.get(ctx, r.dbx, id)
}

func (r *AreaRegistry) List(ctx context.Context) ([]*models.Area, error) {
	var areas []*models.Area

	// Query the database for all locations (atomic operation)
	for area, err := range ScanEntities[models.Area](ctx, r.dbx, r.tableNames.Areas()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list areas")
		}
		areas = append(areas, &area)
	}

	return areas, nil
}

func (r *AreaRegistry) Update(ctx context.Context, area models.Area) (*models.Area, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the area exists
	_, err = r.get(ctx, tx, area.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	// Check if the location exists
	_, err = r.getLocation(ctx, tx, area.LocationID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get location")
	}

	// TODO: what if location has changed, allow or not? (currently allowed)

	err = UpdateEntityByField(ctx, tx, r.tableNames.Areas(), "id", area.ID, area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update area")
	}

	return &area, nil
}

func (r *AreaRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the area exists
	_, err = r.get(ctx, tx, id)
	if err != nil {
		return err
	}

	// Check if the area has commodities
	commodities, err := r.getCommodities(ctx, tx, id)
	if err != nil {
		return err
	}
	if len(commodities) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "area has commodities")
	}

	// Finally, delete the area
	err = DeleteEntityByField(ctx, tx, r.tableNames.Areas(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area")
	}

	return nil
}

func (r *AreaRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := CountEntities(ctx, r.dbx, r.tableNames.Areas())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count areas")
	}

	return cnt, nil
}

func (r *AreaRegistry) AddCommodity(ctx context.Context, areaID, commodityID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the area exists
	_, err = r.get(ctx, tx, areaID)
	if err != nil {
		return errkit.Wrap(err, "failed to get area")
	}

	// Check if the commodity exists
	var commodity models.Commodity
	err = ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", commodityID, &commodity)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodity")
	}

	// Check if the commodity is already in the area
	if commodity.AreaID == areaID {
		// already in area id
		return nil
	}

	// Set the commodity's area ID and update it
	commodity.AreaID = areaID
	err = UpdateEntityByField(ctx, tx, r.tableNames.Areas(), "id", areaID, commodity)
	if err != nil {
		return errkit.Wrap(err, "failed to update commodity")
	}

	return nil
}

func (r *AreaRegistry) GetCommodities(ctx context.Context, areaID string) ([]string, error) {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	return r.getCommodities(ctx, tx, areaID)
}

func (r *AreaRegistry) getCommodities(ctx context.Context, tx sqlx.ExtContext, areaID string) ([]string, error) {
	// Check if the area exists
	_, err := r.get(ctx, tx, areaID)
	if err != nil {
		return nil, err
	}

	var commodities []string

	for commodity, err := range ScanEntitiesByField[models.Commodity](ctx, tx, r.tableNames.Commodities(), "area_id", areaID) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list commodities")
		}
		commodities = append(commodities, commodity.GetID())
	}

	return commodities, nil
}

func (r *AreaRegistry) DeleteCommodity(ctx context.Context, areaID, commodityID string) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the area exists
	_, err = r.get(ctx, tx, areaID)
	if err != nil {
		return err
	}

	var commodity models.Commodity
	err = ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", commodityID, &commodity)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodity")
	}

	if commodity.AreaID != areaID {
		return errkit.Wrap(registry.ErrNotFound, "commodity not found or does not belong to this area")
	}

	err = DeleteEntityByField(ctx, tx, r.tableNames.Commodities(), "id", commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity")
	}

	return nil
}

func (r *AreaRegistry) get(ctx context.Context, tx sqlx.ExtContext, id string) (*models.Area, error) {
	var area models.Area
	err := ScanEntityByField(ctx, tx, r.tableNames.Areas(), "id", id, &area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	return &area, nil
}

func (r *AreaRegistry) getLocation(ctx context.Context, tx sqlx.ExtContext, id string) (*models.Location, error) {
	var location models.Location
	err := ScanEntityByField(ctx, tx, r.tableNames.Locations(), "id", id, &location)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get location")
	}

	return &location, nil
}
