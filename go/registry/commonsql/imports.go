package commonsql

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ImportRegistry = (*ImportRegistry)(nil)

type ImportRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewImportRegistry(dbx *sqlx.DB) *ImportRegistry {
	return NewImportRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewImportRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *ImportRegistry {
	return &ImportRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *ImportRegistry) Create(ctx context.Context, import_ models.Import) (*models.Import, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Generate a new ID
	import_.SetID(generateID())

	// Insert the import
	err = InsertEntity(ctx, tx, r.tableNames.Imports(), import_)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert import")
	}

	return &import_, nil
}

func (r *ImportRegistry) Get(ctx context.Context, id string) (*models.Import, error) {
	var import_ models.Import
	err := ScanEntityByField(ctx, r.dbx, r.tableNames.Imports(), "id", id, &import_)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get import")
	}
	return &import_, nil
}

func (r *ImportRegistry) List(ctx context.Context) ([]*models.Import, error) {
	var imports []*models.Import

	// Query the database for all imports (atomic operation)
	for import_, err := range ScanEntities[models.Import](ctx, r.dbx, r.tableNames.Imports()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list imports")
		}
		imports = append(imports, &import_)
	}

	return imports, nil
}

func (r *ImportRegistry) Update(ctx context.Context, import_ models.Import) (*models.Import, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Update the import
	err = UpdateEntityByField(ctx, tx, r.tableNames.Imports(), "id", import_.ID, import_)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update import")
	}

	return &import_, nil
}

func (r *ImportRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Delete the import
	err = DeleteEntityByField(ctx, tx, r.tableNames.Imports(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete import")
	}

	return nil
}

func (r *ImportRegistry) Count(ctx context.Context) (int, error) {
	return CountEntities(ctx, r.dbx, r.tableNames.Imports())
}
