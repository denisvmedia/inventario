package commonsql

import (
	"context"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ExportRegistry = (*ExportRegistry)(nil)

type ExportRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewExportRegistry(dbx *sqlx.DB) *ExportRegistry {
	return NewExportRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewExportRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *ExportRegistry {
	return &ExportRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *ExportRegistry) Create(ctx context.Context, export models.Export) (*models.Export, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Generate a new ID
	export.SetID(generateID())

	// Set created date if not set
	if export.CreatedDate == nil {
		now := models.Date(time.Now().Format("2006-01-02"))
		export.CreatedDate = &now
	}

	// Insert the export
	err = InsertEntity(ctx, tx, r.tableNames.Exports(), export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert export")
	}

	return &export, nil
}

func (r *ExportRegistry) Get(ctx context.Context, id string) (*models.Export, error) {
	var export models.Export
	err := ScanEntityByField(ctx, r.dbx, r.tableNames.Exports(), "id", id, &export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get export")
	}

	return &export, nil
}

func (r *ExportRegistry) List(ctx context.Context) ([]*models.Export, error) {
	var exports []*models.Export

	// Query the database for all exports (atomic operation)
	for export, err := range ScanEntities[models.Export](ctx, r.dbx, r.tableNames.Exports()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list exports")
		}
		exports = append(exports, &export)
	}

	return exports, nil
}

func (r *ExportRegistry) Update(ctx context.Context, export models.Export) (*models.Export, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Update the export
	err = UpdateEntityByField(ctx, tx, r.tableNames.Exports(), "id", export.ID, export)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update export")
	}

	return &export, nil
}

func (r *ExportRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Delete the export
	err = DeleteEntityByField(ctx, tx, r.tableNames.Exports(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete export")
	}

	return nil
}

func (r *ExportRegistry) Count(ctx context.Context) (int, error) {
	count, err := CountEntities(ctx, r.dbx, r.tableNames.Exports())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count exports")
	}

	return count, nil
}
