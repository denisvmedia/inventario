package commonsql

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ManualRegistry = (*ManualRegistry)(nil)

type ManualRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewManualRegistry(dbx *sqlx.DB) *ManualRegistry {
	return NewManualRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewManualRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *ManualRegistry {
	return &ManualRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *ManualRegistry) Create(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	var commodity models.Commodity
	err = ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", manual.CommodityID, &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// Generate a new ID
	manual.SetID(generateID())

	err = InsertEntity(ctx, tx, r.tableNames.Manuals(), manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &manual, nil
}

func (r *ManualRegistry) Get(ctx context.Context, id string) (*models.Manual, error) {
	return r.get(ctx, r.dbx, id)
}

func (r *ManualRegistry) List(ctx context.Context) ([]*models.Manual, error) {
	var manuals []*models.Manual

	// Query the database for all manuals (atomic operation)
	for manual, err := range ScanEntities[models.Manual](ctx, r.dbx, r.tableNames.Manuals()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list manuals")
		}
		manuals = append(manuals, &manual)
	}

	return manuals, nil
}

func (r *ManualRegistry) Update(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(RollbackOrCommit(tx, err))
	}()

	// Check if the manual exists
	_, err = r.get(ctx, tx, manual.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get manual")
	}

	// Check if the commodity exists
	_, err = r.getCommodity(ctx, tx, manual.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// TODO: what if commodity has changed, allow or not? (currently allowed)

	err = UpdateEntityByField(ctx, tx, r.tableNames.Manuals(), "id", manual.ID, manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update manual")
	}

	return &manual, nil
}

func (r *ManualRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(RollbackOrCommit(tx, err))
	}()

	// Check if the manual exists
	_, err = r.get(ctx, tx, id)
	if err != nil {
		return err
	}

	// Finally, delete the manual
	err = DeleteEntityByField(ctx, tx, r.tableNames.Manuals(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual")
	}

	return nil
}

func (r *ManualRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := CountEntities(ctx, r.dbx, r.tableNames.Manuals())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count manuals")
	}

	return cnt, nil
}

func (r *ManualRegistry) get(ctx context.Context, tx sqlx.ExtContext, id string) (*models.Manual, error) {
	var manual models.Manual
	err := ScanEntityByField(ctx, tx, r.tableNames.Manuals(), "id", id, &manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get manual")
	}

	return &manual, nil
}

func (r *ManualRegistry) getCommodity(ctx context.Context, tx sqlx.ExtContext, commodityID string) (*models.Commodity, error) {
	var commodity models.Commodity
	err := ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", commodityID, &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	return &commodity, nil
}
